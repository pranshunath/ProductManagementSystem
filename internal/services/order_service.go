package services

import (
	"errors"
	"fmt"
	"math"
	"time"

	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/pkg/response"

	"gorm.io/gorm"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrUnauthorizedOrder  = errors.New("you do not have permission to access this order")
	ErrEmptyOrder         = errors.New("order must contain at least one item")
	ErrInvalidOrderState  = errors.New("illegal order state transition")
	ErrCannotCancelOrder  = errors.New("order cannot be cancelled in its current state")
)

// OrderItemInput defines payload for checkout items
type OrderItemInput struct {
	ProductID uint `json:"product_id"`
	Quantity  int  `json:"quantity"`
}

// OrderService defines contract for order execution and lifecycle management
type OrderService interface {
	CreateOrder(userID uint, items []OrderItemInput, idempotencyKey string) (*models.Order, error)
	GetOrderByID(id uint, requestingUserID uint, isAdmin bool) (*models.Order, error)
	ListUserOrders(userID uint, page, limit int) ([]models.Order, *response.Pagination, error)
	ListAllOrders(page, limit int, status models.OrderStatus) ([]models.Order, *response.Pagination, error)
	CancelOrder(orderID uint, requestingUserID uint, isAdmin bool) (*models.Order, error)
	UpdateOrderStatus(orderID uint, nextStatus models.OrderStatus) (*models.Order, error)
	GetDashboardMetrics() (map[string]interface{}, error)
}

type orderService struct {
	orderRepo repositories.OrderRepository
	invRepo   repositories.InventoryRepository
	prodRepo  repositories.ProductRepository
	userRepo  repositories.UserRepository
}

// NewOrderService returns an instance of OrderService
func NewOrderService(
	orderRepo repositories.OrderRepository,
	invRepo repositories.InventoryRepository,
	prodRepo repositories.ProductRepository,
	userRepo repositories.UserRepository,
) OrderService {
	return &orderService{
		orderRepo: orderRepo,
		invRepo:   invRepo,
		prodRepo:  prodRepo,
		userRepo:  userRepo,
	}
}

// CreateOrder executes the complete checkout flow inside an ACID transaction with atomic stock reservation
func (s *orderService) CreateOrder(userID uint, items []OrderItemInput, idempotencyKey string) (*models.Order, error) {
	if len(items) == 0 {
		return nil, ErrEmptyOrder
	}

	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, errors.New("item quantity must be greater than zero")
		}
	}

	db := s.orderRepo.GetDB()
	var createdOrder models.Order

	// Execute complete checkout inside a single database transaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var totalAmount float64
		orderItems := make([]models.OrderItem, 0, len(items))

		orderRefID := fmt.Sprintf("ORD-PRE-%d-%d", userID, time.Now().UnixNano())

		// Process each item with row locking and atomic checks
		for _, input := range items {
			var product models.Product

			// SELECT ... FOR UPDATE on product to acquire row lock and prevent race condition
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&product, input.ProductID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("product ID %d not found", input.ProductID)
				}
				return err
			}

			// Validate product status
			if product.Status != models.ProductStatusActive {
				return fmt.Errorf("product '%s' is not available for purchase", product.Name)
			}

			// Atomic Stock Reservation: verifies (stock - reserved_stock) >= quantity
			if err := s.invRepo.ReserveStockAtomic(tx, product.ID, input.Quantity, orderRefID); err != nil {
				return fmt.Errorf("insufficient stock for product '%s' (requested %d, available %d)",
					product.Name, input.Quantity, product.AvailableStock())
			}

			subtotal := product.Price * float64(input.Quantity)
			totalAmount += subtotal

			orderItems = append(orderItems, models.OrderItem{
				ProductID: product.ID,
				Quantity:  input.Quantity,
				UnitPrice: product.Price,
				Subtotal:  subtotal,
			})
		}

		// Create Order entity in PENDING status
		order := models.Order{
			UserID:      userID,
			Status:      models.OrderStatusPending,
			TotalAmount: totalAmount,
			Items:       orderItems,
		}

		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		// Immediately confirm stock reservation and commit to CONFIRMED state
		finalRefID := fmt.Sprintf("ORDER-%d", order.ID)
		for _, item := range order.Items {
			if err := s.invRepo.CommitReservedStockAtomic(tx, item.ProductID, item.Quantity, finalRefID); err != nil {
				return err
			}
		}

		// Update order status to CONFIRMED
		order.Status = models.OrderStatusConfirmed
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		createdOrder = order
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Fetch full order with preloaded relations
	return s.orderRepo.GetByID(createdOrder.ID)
}

func (s *orderService) GetOrderByID(id uint, requestingUserID uint, isAdmin bool) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}

	if !isAdmin && order.UserID != requestingUserID {
		return nil, ErrUnauthorizedOrder
	}

	return order, nil
}

func (s *orderService) ListUserOrders(userID uint, page, limit int) ([]models.Order, *response.Pagination, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	orders, total, err := s.orderRepo.ListByUserID(userID, page, limit)
	if err != nil {
		return nil, nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	pagination := &response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	return orders, pagination, nil
}

func (s *orderService) ListAllOrders(page, limit int, status models.OrderStatus) ([]models.Order, *response.Pagination, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	orders, total, err := s.orderRepo.ListAll(page, limit, status)
	if err != nil {
		return nil, nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	pagination := &response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	return orders, pagination, nil
}

// CancelOrder releases reserved or deducted stock and sets status to CANCELLED
func (s *orderService) CancelOrder(orderID uint, requestingUserID uint, isAdmin bool) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}

	if !isAdmin && order.UserID != requestingUserID {
		return nil, ErrUnauthorizedOrder
	}

	// Validate order lifecycle transition
	if err := order.ValidateTransition(models.OrderStatusCancelled); err != nil {
		return nil, ErrCannotCancelOrder
	}

	db := s.orderRepo.GetDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		// Restock items back into inventory
		cancelRef := fmt.Sprintf("CANCEL-ORDER-%d", order.ID)
		for _, item := range order.Items {
			// Increment stock and record STOCK_IN
			if err := tx.Model(&models.Product{}).
				Where("id = ?", item.ProductID).
				Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
				return err
			}

			invTx := models.InventoryTransaction{
				ProductID:   item.ProductID,
				Type:        models.TxTypeStockIn,
				Quantity:    item.Quantity,
				ReferenceID: cancelRef,
				Notes:       fmt.Sprintf("Restocked from cancelled order #%d", order.ID),
				CreatedAt:   time.Now().UTC(),
			}
			if err := tx.Create(&invTx).Error; err != nil {
				return err
			}
		}

		return s.orderRepo.UpdateStatus(tx, order.ID, models.OrderStatusCancelled)
	})

	if err != nil {
		return nil, err
	}

	return s.orderRepo.GetByID(order.ID)
}

func (s *orderService) UpdateOrderStatus(orderID uint, nextStatus models.OrderStatus) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}

	if err := order.ValidateTransition(nextStatus); err != nil {
		return nil, err
	}

	if err := s.orderRepo.UpdateStatus(nil, order.ID, nextStatus); err != nil {
		return nil, err
	}

	return s.orderRepo.GetByID(order.ID)
}

func (s *orderService) GetDashboardMetrics() (map[string]interface{}, error) {
	totalOrders, err := s.orderRepo.CountTotal()
	if err != nil {
		return nil, err
	}

	pendingOrders, _ := s.orderRepo.CountByStatus(models.OrderStatusPending)
	confirmedOrders, _ := s.orderRepo.CountByStatus(models.OrderStatusConfirmed)
	shippedOrders, _ := s.orderRepo.CountByStatus(models.OrderStatusShipped)
	deliveredOrders, _ := s.orderRepo.CountByStatus(models.OrderStatusDelivered)
	cancelledOrders, _ := s.orderRepo.CountByStatus(models.OrderStatusCancelled)

	revenue, _ := s.orderRepo.TotalRevenue()
	customersCount, _ := s.userRepo.CountCustomers()
	productsCount, _ := s.prodRepo.CountTotal()
	lowStockCount, _ := s.prodRepo.CountLowStock(10)
	outOfStockCount, _ := s.prodRepo.CountOutOfStock()

	recentOrders, _ := s.orderRepo.GetRecentOrders(5)

	return map[string]interface{}{
		"total_orders":       totalOrders,
		"pending_orders":     pendingOrders,
		"confirmed_orders":   confirmedOrders,
		"shipped_orders":     shippedOrders,
		"delivered_orders":   deliveredOrders,
		"cancelled_orders":   cancelledOrders,
		"total_revenue":      revenue,
		"total_customers":    customersCount,
		"total_products":     productsCount,
		"low_stock_count":    lowStockCount,
		"out_of_stock_count": outOfStockCount,
		"recent_orders":      recentOrders,
	}, nil
}
