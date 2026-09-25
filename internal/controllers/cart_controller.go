package controllers

import (
	"errors"
	"strconv"

	"producthub/internal/middleware"
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/internal/validators"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// CartController handles shopping cart operations for authenticated users
type CartController struct {
	cartService services.CartService
}

// NewCartController creates a new instance of CartController
func NewCartController(cartService services.CartService) *CartController {
	return &CartController{cartService: cartService}
}

// GetCart handles GET /api/cart
func (ctrl *CartController) GetCart(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	cart, err := ctrl.cartService.GetCart(userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, cart)
}

// AddItem handles POST /api/cart/items
func (ctrl *CartController) AddItem(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req validators.AddCartItemRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	cart, err := ctrl.cartService.AddItem(userID, req.ProductID, req.Quantity)
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		if errors.Is(err, repositories.ErrInsufficientStock) {
			return response.Error(c, fiber.StatusBadRequest, "INSUFFICIENT_STOCK", "Requested quantity exceeds available stock")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, cart)
}

// UpdateItem handles PUT /api/cart/items/:product_id
func (ctrl *CartController) UpdateItem(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	prodIDParam := c.Params("product_id")
	productID, err := strconv.ParseUint(prodIDParam, 10, 32)
	if err != nil || productID == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	var req validators.UpdateCartItemRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	cart, err := ctrl.cartService.UpdateQuantity(userID, uint(productID), req.Quantity)
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		if errors.Is(err, repositories.ErrInsufficientStock) {
			return response.Error(c, fiber.StatusBadRequest, "INSUFFICIENT_STOCK", "Requested quantity exceeds available stock")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, cart)
}

// RemoveItem handles DELETE /api/cart/items/:product_id
func (ctrl *CartController) RemoveItem(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	prodIDParam := c.Params("product_id")
	productID, err := strconv.ParseUint(prodIDParam, 10, 32)
	if err != nil || productID == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	cart, err := ctrl.cartService.RemoveItem(userID, uint(productID))
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, cart)
}

// ClearCart handles DELETE /api/cart
func (ctrl *CartController) ClearCart(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	if err := ctrl.cartService.ClearCart(userID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Cart cleared successfully",
	})
}
