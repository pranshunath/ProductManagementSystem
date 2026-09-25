package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"producthub/internal/api"
	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/pkg/jwt"
	"producthub/pkg/response"

	"gorm.io/gorm"
)

// MockCartRepo implements repositories.CartRepository for unit testing
type MockCartRepo struct {
	carts  map[uint]*models.Cart
	nextID uint
}

func NewMockCartRepo() *MockCartRepo {
	return &MockCartRepo{
		carts:  make(map[uint]*models.Cart),
		nextID: 1,
	}
}

func (m *MockCartRepo) GetByUserID(userID uint) (*models.Cart, error) {
	if cart, ok := m.carts[userID]; ok {
		return cart, nil
	}
	newCart := &models.Cart{
		ID:        m.nextID,
		UserID:    userID,
		Items:     []models.CartItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.nextID++
	m.carts[userID] = newCart
	return newCart, nil
}

func (m *MockCartRepo) AddItem(userID uint, productID uint, quantity int) (*models.Cart, error) {
	cart, _ := m.GetByUserID(userID)
	for i := range cart.Items {
		if cart.Items[i].ProductID == productID {
			cart.Items[i].Quantity += quantity
			return cart, nil
		}
	}
	cart.Items = append(cart.Items, models.CartItem{
		ID:        uint(len(cart.Items) + 1),
		CartID:    cart.ID,
		ProductID: productID,
		Quantity:  quantity,
	})
	return cart, nil
}

func (m *MockCartRepo) UpdateItemQuantity(userID uint, productID uint, quantity int) (*models.Cart, error) {
	cart, _ := m.GetByUserID(userID)
	if quantity <= 0 {
		return m.RemoveItem(userID, productID)
	}
	for i := range cart.Items {
		if cart.Items[i].ProductID == productID {
			cart.Items[i].Quantity = quantity
			return cart, nil
		}
	}
	return cart, nil
}

func (m *MockCartRepo) RemoveItem(userID uint, productID uint) (*models.Cart, error) {
	cart, _ := m.GetByUserID(userID)
	filtered := make([]models.CartItem, 0, len(cart.Items))
	for _, item := range cart.Items {
		if item.ProductID != productID {
			filtered = append(filtered, item)
		}
	}
	cart.Items = filtered
	return cart, nil
}

func (m *MockCartRepo) ClearCart(userID uint) error {
	cart, _ := m.GetByUserID(userID)
	cart.Items = []models.CartItem{}
	return nil
}

// MockOrderRepo implements repositories.OrderRepository for unit testing
type MockOrderRepo struct {
	orders map[uint]*models.Order
	nextID uint
}

func NewMockOrderRepo() *MockOrderRepo {
	return &MockOrderRepo{
		orders: make(map[uint]*models.Order),
		nextID: 1,
	}
}

func (m *MockOrderRepo) GetDB() *gorm.DB {
	return nil
}

func (m *MockOrderRepo) GetByID(id uint) (*models.Order, error) {
	if o, ok := m.orders[id]; ok {
		return o, nil
	}
	return nil, nil
}

func (m *MockOrderRepo) Create(tx *gorm.DB, order *models.Order) error {
	order.ID = m.nextID
	m.nextID++
	order.CreatedAt = time.Now().UTC()
	order.UpdatedAt = time.Now().UTC()
	m.orders[order.ID] = order
	return nil
}

func (m *MockOrderRepo) UpdateStatus(tx *gorm.DB, id uint, status models.OrderStatus) error {
	if o, ok := m.orders[id]; ok {
		o.Status = status
		o.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (m *MockOrderRepo) ListByUserID(userID uint, page, limit int) ([]models.Order, int64, error) {
	var list []models.Order
	for _, o := range m.orders {
		if o.UserID == userID {
			list = append(list, *o)
		}
	}
	return list, int64(len(list)), nil
}

func (m *MockOrderRepo) ListAll(page, limit int, status models.OrderStatus) ([]models.Order, int64, error) {
	var list []models.Order
	for _, o := range m.orders {
		if status == "" || o.Status == status {
			list = append(list, *o)
		}
	}
	return list, int64(len(list)), nil
}

func (m *MockOrderRepo) CountTotal() (int64, error) {
	return int64(len(m.orders)), nil
}

func (m *MockOrderRepo) CountByStatus(status models.OrderStatus) (int64, error) {
	var count int64
	for _, o := range m.orders {
		if o.Status == status {
			count++
		}
	}
	return count, nil
}

func (m *MockOrderRepo) TotalRevenue() (float64, error) {
	var rev float64
	for _, o := range m.orders {
		if o.Status != models.OrderStatusCancelled {
			rev += o.TotalAmount
		}
	}
	return rev, nil
}

func (m *MockOrderRepo) GetRecentOrders(limit int) ([]models.Order, error) {
	var list []models.Order
	for _, o := range m.orders {
		list = append(list, *o)
	}
	return list, nil
}

// In-Memory Order Service for Unit Tests
type InMemOrderService struct {
	orderRepo *MockOrderRepo
	prodRepo  repositories.ProductRepository
}

func (s *InMemOrderService) CreateOrder(userID uint, items []services.OrderItemInput, idempotencyKey string) (*models.Order, error) {
	if len(items) == 0 {
		return nil, services.ErrEmptyOrder
	}

	var totalAmount float64
	orderItems := make([]models.OrderItem, 0, len(items))

	for _, it := range items {
		p, err := s.prodRepo.GetByID(it.ProductID)
		if err != nil || p == nil {
			return nil, fmt.Errorf("product ID %d not found", it.ProductID)
		}
		if p.AvailableStock() < it.Quantity {
			return nil, fmt.Errorf("insufficient stock for product '%s'", p.Name)
		}

		p.Stock -= it.Quantity
		subtotal := p.Price * float64(it.Quantity)
		totalAmount += subtotal

		orderItems = append(orderItems, models.OrderItem{
			ProductID: it.ProductID,
			Product:   *p,
			Quantity:  it.Quantity,
			UnitPrice: p.Price,
			Subtotal:  subtotal,
		})
	}

	order := &models.Order{
		UserID:      userID,
		Status:      models.OrderStatusConfirmed,
		TotalAmount: totalAmount,
		Items:       orderItems,
	}

	_ = s.orderRepo.Create(nil, order)
	return order, nil
}

func (s *InMemOrderService) GetOrderByID(id uint, requestingUserID uint, isAdmin bool) (*models.Order, error) {
	o, err := s.orderRepo.GetByID(id)
	if err != nil || o == nil {
		return nil, services.ErrOrderNotFound
	}
	if !isAdmin && o.UserID != requestingUserID {
		return nil, services.ErrUnauthorizedOrder
	}
	return o, nil
}

func (s *InMemOrderService) ListUserOrders(userID uint, page, limit int) ([]models.Order, *response.Pagination, error) {
	orders, total, _ := s.orderRepo.ListByUserID(userID, page, limit)
	return orders, &response.Pagination{Page: page, Limit: limit, Total: total, TotalPages: 1}, nil
}

func (s *InMemOrderService) ListAllOrders(page, limit int, status models.OrderStatus) ([]models.Order, *response.Pagination, error) {
	orders, total, _ := s.orderRepo.ListAll(page, limit, status)
	return orders, &response.Pagination{Page: page, Limit: limit, Total: total, TotalPages: 1}, nil
}

func (s *InMemOrderService) CancelOrder(orderID uint, requestingUserID uint, isAdmin bool) (*models.Order, error) {
	o, err := s.orderRepo.GetByID(orderID)
	if err != nil || o == nil {
		return nil, services.ErrOrderNotFound
	}
	if !isAdmin && o.UserID != requestingUserID {
		return nil, services.ErrUnauthorizedOrder
	}
	if err := o.ValidateTransition(models.OrderStatusCancelled); err != nil {
		return nil, services.ErrCannotCancelOrder
	}

	// Restock items
	for _, it := range o.Items {
		p, _ := s.prodRepo.GetByID(it.ProductID)
		if p != nil {
			p.Stock += it.Quantity
		}
	}

	o.Status = models.OrderStatusCancelled
	return o, nil
}

func (s *InMemOrderService) UpdateOrderStatus(orderID uint, nextStatus models.OrderStatus) (*models.Order, error) {
	o, err := s.orderRepo.GetByID(orderID)
	if err != nil || o == nil {
		return nil, services.ErrOrderNotFound
	}
	if err := o.ValidateTransition(nextStatus); err != nil {
		return nil, fmt.Errorf("%w: %v", services.ErrInvalidOrderState, err)
	}
	o.Status = nextStatus
	return o, nil
}

func (s *InMemOrderService) GetDashboardMetrics() (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func TestCartAndOrderFlow(t *testing.T) {
	catRepo := NewMockCategoryRepo()
	prodRepo := NewMockProductRepo()
	cartRepo := NewMockCartRepo()
	orderRepo := NewMockOrderRepo()

	catService := services.NewCategoryService(catRepo)
	prodService := services.NewProductService(prodRepo, catRepo, &MockInventoryRepo{})
	cartService := services.NewCartService(cartRepo, prodRepo)
	orderService := &InMemOrderService{orderRepo: orderRepo, prodRepo: prodRepo}

	testSecret := "super-secure-cart-and-order-test-secret"
	userAToken, _ := jwt.GenerateToken(10, "usera@test.com", models.RoleCustomer, testSecret, 24)
	userBToken, _ := jwt.GenerateToken(20, "userb@test.com", models.RoleCustomer, testSecret, 24)
	adminToken, _ := jwt.GenerateToken(1, "admin@test.com", models.RoleAdmin, testSecret, 24)

	app := api.SetupRouter(api.RouterConfig{
		DB:                 nil,
		JWTSecret:          testSecret,
		JWTExpirationHours: 24,
		CategoryService:    catService,
		ProductService:     prodService,
		CartService:        cartService,
		OrderService:       orderService,
	})

	// Seed catalog
	cat, _ := catService.CreateCategory("Displays", "Computer monitors")
	prod, _ := prodService.CreateProduct("DISP-4K-27", "4K Ultra-Sharp Monitor", "27-inch IPS panel", cat.ID, 499.00, 10)

	// 1. User A adds item to cart (quantity: 2)
	addPayload, _ := json.Marshal(map[string]interface{}{
		"product_id": prod.ID,
		"quantity":   2,
	})
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/cart/items", bytes.NewReader(addPayload))
	reqAdd.Header.Set("Content-Type", "application/json")
	reqAdd.Header.Set("Authorization", "Bearer "+userAToken)

	respAdd, err := app.Test(reqAdd, -1)
	if err != nil || respAdd.StatusCode != http.StatusOK {
		t.Fatalf("Failed to add to cart: %v, status: %d", err, respAdd.StatusCode)
	}

	// 2. User A views cart -> Total items: 2, Total amount: 998.00
	reqGetCart := httptest.NewRequest(http.MethodGet, "/api/cart", nil)
	reqGetCart.Header.Set("Authorization", "Bearer "+userAToken)
	respGetCart, _ := app.Test(reqGetCart, -1)
	if respGetCart.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for User A cart, got %d", respGetCart.StatusCode)
	}

	// 3. User B views cart -> Must be isolated (Total items: 0)
	reqUserBCart := httptest.NewRequest(http.MethodGet, "/api/cart", nil)
	reqUserBCart.Header.Set("Authorization", "Bearer "+userBToken)
	respUserBCart, _ := app.Test(reqUserBCart, -1)
	if respUserBCart.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for User B cart, got %d", respUserBCart.StatusCode)
	}

	var resB response.Response
	bBytes, _ := io.ReadAll(respUserBCart.Body)
	_ = json.Unmarshal(bBytes, &resB)
	bData := resB.Data.(map[string]interface{})
	if bData["total_items"].(float64) != 0 {
		t.Errorf("Cart data leak! Expected User B to have 0 items, got %v", bData["total_items"])
	}

	// 4. User A checks out cart -> Creates order, deducts stock, empties cart
	reqCheckout := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader([]byte("{}")))
	reqCheckout.Header.Set("Content-Type", "application/json")
	reqCheckout.Header.Set("Authorization", "Bearer "+userAToken)

	respCheckout, _ := app.Test(reqCheckout, -1)
	if respCheckout.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created for order checkout, got %d", respCheckout.StatusCode)
	}

	var orderRes response.Response
	ordBytes, _ := io.ReadAll(respCheckout.Body)
	_ = json.Unmarshal(ordBytes, &orderRes)
	orderData := orderRes.Data.(map[string]interface{})
	orderID := uint(orderData["id"].(float64))

	// Verify User A cart is now empty
	cartAfter, _ := cartService.GetCart(10)
	if len(cartAfter.Items) != 0 {
		t.Errorf("Expected cart to be cleared after checkout, got %d items", len(cartAfter.Items))
	}

	// Verify remaining product stock is 8 (10 - 2)
	pUpdated, _ := prodRepo.GetByID(prod.ID)
	if pUpdated.Stock != 8 {
		t.Errorf("Expected remaining stock 8, got %d", pUpdated.Stock)
	}

	// 5. User B attempts to access User A's order -> Expect 403 Forbidden
	reqStealOrder := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/orders/%d", orderID), nil)
	reqStealOrder.Header.Set("Authorization", "Bearer "+userBToken)
	respSteal, _ := app.Test(reqStealOrder, -1)
	if respSteal.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for unauthorized order access, got %d", respSteal.StatusCode)
	}

	// 6. User A cancels order -> Stock restocked to 10
	reqCancel := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/orders/%d/cancel", orderID), nil)
	reqCancel.Header.Set("Authorization", "Bearer "+userAToken)
	respCancel, _ := app.Test(reqCancel, -1)
	if respCancel.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for order cancellation, got %d", respCancel.StatusCode)
	}

	pRestocked, _ := prodRepo.GetByID(prod.ID)
	if pRestocked.Stock != 10 {
		t.Errorf("Expected stock to return to 10 after cancellation, got %d", pRestocked.Stock)
	}

	// 7. Admin updates order status (Illegal transition test: CANCELLED -> DELIVERED rejected)
	updateStatusPayload, _ := json.Marshal(map[string]string{
		"status": "DELIVERED",
	})
	reqBadTransition := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/orders/%d/status", orderID), bytes.NewReader(updateStatusPayload))
	reqBadTransition.Header.Set("Content-Type", "application/json")
	reqBadTransition.Header.Set("Authorization", "Bearer "+adminToken)

	respBadTransition, _ := app.Test(reqBadTransition, -1)
	if respBadTransition.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 Unprocessable Entity for invalid state transition, got %d", respBadTransition.StatusCode)
	}
}
