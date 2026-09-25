package controllers

import (
	"errors"
	"fmt"
	"strconv"

	"producthub/internal/cache"
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/internal/validators"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// InventoryController handles HTTP requests for stock management and audit trails
type InventoryController struct {
	invService   services.InventoryService
	prodService  services.ProductService
	cacheService cache.CacheService
}

// NewInventoryController creates a new instance of InventoryController
func NewInventoryController(invService services.InventoryService, prodService services.ProductService, cacheService cache.CacheService) *InventoryController {
	return &InventoryController{
		invService:   invService,
		prodService:  prodService,
		cacheService: cacheService,
	}
}

// Restock handles POST /api/inventory/restock (Admin only) and invalidates product cache
func (ctrl *InventoryController) Restock(c *fiber.Ctx) error {
	var req validators.RestockRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	product, err := ctrl.invService.RestockProduct(req.ProductID, req.Quantity, req.Notes)
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	if product == nil {
		return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
	}

	// Invalidate Cache-Aside entry for restocked product
	if ctrl.cacheService != nil {
		_ = ctrl.cacheService.Delete(c.Context(), fmt.Sprintf("product:id:%d", product.ID))
	}

	return response.Success(c, fiber.Map{
		"message":         "Product successfully restocked",
		"product_id":      product.ID,
		"sku":             product.SKU,
		"name":            product.Name,
		"stock":           product.Stock,
		"reserved_stock":  product.ReservedStock,
		"available_stock": product.AvailableStock(),
		"stock_status":    product.StockStatus(),
	})
}

// Summary handles GET /api/inventory/summary (Admin only)
func (ctrl *InventoryController) Summary(c *fiber.Ctx) error {
	metrics, err := ctrl.invService.GetInventorySummary()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
	return response.Success(c, metrics)
}

// Transactions handles GET /api/inventory/transactions (Admin only)
func (ctrl *InventoryController) Transactions(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	txs, pagination, err := ctrl.invService.ListAllAuditTrails(page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Paginated(c, fiber.StatusOK, txs, pagination)
}

// ProductTransactions handles GET /api/inventory/products/:id/transactions (Admin only)
func (ctrl *InventoryController) ProductTransactions(c *fiber.Ctx) error {
	idParam := c.Params("id")
	productID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || productID == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	txs, pagination, err := ctrl.invService.GetProductAuditTrail(uint(productID), page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Paginated(c, fiber.StatusOK, txs, pagination)
}

// GetStock handles GET /api/inventory/products/:id/stock (Public)
func (ctrl *InventoryController) GetStock(c *fiber.Ctx) error {
	idParam := c.Params("id")
	productID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || productID == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	prod, err := ctrl.prodService.GetProductByID(uint(productID))
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, fiber.Map{
		"product_id":      prod.ID,
		"sku":             prod.SKU,
		"name":            prod.Name,
		"stock":           prod.Stock,
		"reserved_stock":  prod.ReservedStock,
		"available_stock": prod.AvailableStock(),
		"stock_status":    prod.StockStatus(),
	})
}

// CheckStock handles POST /api/inventory/check-stock (Public)
func (ctrl *InventoryController) CheckStock(c *fiber.Ctx) error {
	var req validators.CheckStockBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	canFulfill, avail, err := ctrl.invService.CheckStock(req.ProductID, req.Quantity)
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, fiber.Map{
		"product_id":      req.ProductID,
		"requested":       req.Quantity,
		"available_stock": avail,
		"can_fulfill":     canFulfill,
	})
}
