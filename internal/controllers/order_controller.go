package controllers

import (
	"errors"
	"strconv"
	"strings"

	"producthub/internal/middleware"
	"producthub/internal/models"
	"producthub/internal/services"
	"producthub/internal/validators"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// OrderController handles customer purchases and admin order processing
type OrderController struct {
	orderService services.OrderService
	cartService  services.CartService
}

// NewOrderController creates a new instance of OrderController
func NewOrderController(orderService services.OrderService, cartService services.CartService) *OrderController {
	return &OrderController{
		orderService: orderService,
		cartService:  cartService,
	}
}

// Create handles POST /api/orders (Checkout)
// If items are omitted in the request body, automatically checks out the user's active cart.
func (ctrl *OrderController) Create(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req validators.CreateOrderRequest
	_ = c.BodyParser(&req) // Optional body parsing

	idempotencyKey := strings.TrimSpace(c.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = req.IdempotencyKey
	}

	items := req.Items
	checkedOutFromCart := false

	// If no explicit items provided, checkout items from active shopping cart
	if len(items) == 0 {
		cart, err := ctrl.cartService.GetCart(userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve shopping cart")
		}

		if len(cart.Items) == 0 {
			return response.Error(c, fiber.StatusBadRequest, "EMPTY_CART", "Your cart is empty. Add products to cart before checkout.")
		}

		items = make([]services.OrderItemInput, len(cart.Items))
		for i, ci := range cart.Items {
			items[i] = services.OrderItemInput{
				ProductID: ci.ProductID,
				Quantity:  ci.Quantity,
			}
		}
		checkedOutFromCart = true
	} else {
		if errs := req.Validate(); errs.HasErrors() {
			return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
		}
	}

	// Execute complete checkout transaction
	order, err := ctrl.orderService.CreateOrder(userID, items, idempotencyKey)
	if err != nil {
		if errors.Is(err, services.ErrEmptyOrder) {
			return response.Error(c, fiber.StatusBadRequest, "EMPTY_ORDER", err.Error())
		}
		if strings.Contains(err.Error(), "insufficient stock") {
			return response.Error(c, fiber.StatusBadRequest, "INSUFFICIENT_STOCK", err.Error())
		}
		if strings.Contains(err.Error(), "not found") {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", err.Error())
		}
		return response.Error(c, fiber.StatusInternalServerError, "CHECKOUT_FAILED", err.Error())
	}

	// If checkout succeeded from cart, clear the cart
	if checkedOutFromCart {
		_ = ctrl.cartService.ClearCart(userID)
	}

	return response.Created(c, order)
}

// ListMyOrders handles GET /api/orders (Customer view)
func (ctrl *OrderController) ListMyOrders(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	orders, pagination, err := ctrl.orderService.ListUserOrders(userID, page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Paginated(c, fiber.StatusOK, orders, pagination)
}

// GetByID handles GET /api/orders/:id
func (ctrl *OrderController) GetByID(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	idParam := c.Params("id")
	orderID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || orderID == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Order ID must be a positive integer")
	}

	isAdmin := middleware.IsAdmin(c)
	order, err := ctrl.orderService.GetOrderByID(uint(orderID), userID, isAdmin)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			return response.Error(c, fiber.StatusNotFound, "ORDER_NOT_FOUND", "Order not found")
		}
		if errors.Is(err, services.ErrUnauthorizedOrder) {
			return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "You do not have access to this order")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, order)
}

// Cancel handles POST /api/orders/:id/cancel
func (ctrl *OrderController) Cancel(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	idParam := c.Params("id")
	orderID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || orderID == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Order ID must be a positive integer")
	}

	isAdmin := middleware.IsAdmin(c)
	order, err := ctrl.orderService.CancelOrder(uint(orderID), userID, isAdmin)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			return response.Error(c, fiber.StatusNotFound, "ORDER_NOT_FOUND", "Order not found")
		}
		if errors.Is(err, services.ErrUnauthorizedOrder) {
			return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to cancel this order")
		}
		if errors.Is(err, services.ErrCannotCancelOrder) {
			return response.Error(c, fiber.StatusUnprocessableEntity, "CANNOT_CANCEL", "Order cannot be cancelled in its current state")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, order)
}

// ListAll handles GET /api/admin/orders (Admin view)
func (ctrl *OrderController) ListAll(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	statusFilter := models.OrderStatus(strings.ToUpper(strings.TrimSpace(c.Query("status"))))

	orders, pagination, err := ctrl.orderService.ListAllOrders(page, limit, statusFilter)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Paginated(c, fiber.StatusOK, orders, pagination)
}

// UpdateStatus handles PUT /api/admin/orders/:id/status (Admin view)
func (ctrl *OrderController) UpdateStatus(c *fiber.Ctx) error {
	idParam := c.Params("id")
	orderID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || orderID == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Order ID must be a positive integer")
	}

	var req validators.UpdateOrderStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	targetStatus := models.OrderStatus(strings.ToUpper(strings.TrimSpace(req.Status)))
	order, err := ctrl.orderService.UpdateOrderStatus(uint(orderID), targetStatus)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			return response.Error(c, fiber.StatusNotFound, "ORDER_NOT_FOUND", "Order not found")
		}
		if strings.Contains(err.Error(), "invalid order state transition") {
			return response.Error(c, fiber.StatusUnprocessableEntity, "INVALID_STATE_TRANSITION", err.Error())
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, order)
}
