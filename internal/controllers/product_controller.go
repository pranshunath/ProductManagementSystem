package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"producthub/internal/cache"
	"producthub/internal/grpc/clients"
	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/internal/validators"
	"producthub/pkg/pb"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProductController handles HTTP requests for product catalog
type ProductController struct {
	prodService  services.ProductService
	grpcClients  *clients.GRPCClients
	cacheService cache.CacheService
}

// NewProductController creates a new instance of ProductController
func NewProductController(prodService services.ProductService, grpcClients *clients.GRPCClients, cacheService cache.CacheService) *ProductController {
	return &ProductController{
		prodService:  prodService,
		grpcClients:  grpcClients,
		cacheService: cacheService,
	}
}

// Create handles POST /api/products
func (ctrl *ProductController) Create(c *fiber.Ctx) error {
	var req validators.CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	product, err := ctrl.prodService.CreateProduct(req.SKU, req.Name, req.Description, req.CategoryID, req.Price, req.Stock)
	if err != nil {
		if errors.Is(err, services.ErrSKUAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "SKU_ALREADY_EXISTS", err.Error())
		}
		if errors.Is(err, services.ErrCategoryNotFound) {
			return response.Error(c, fiber.StatusBadRequest, "CATEGORY_NOT_FOUND", "Specified category does not exist")
		}
		if errors.Is(err, services.ErrInvalidPrice) || errors.Is(err, services.ErrInvalidQuantity) {
			return response.Error(c, fiber.StatusBadRequest, "INVALID_VALUE", err.Error())
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Created(c, product.ToResponse())
}

// List handles GET /api/products with search, filtering, sorting, and pagination
// Dispatches call via internal gRPC client when available
func (ctrl *ProductController) List(c *fiber.Ctx) error {
	filter, errs := validators.ParseProductFilter(c)
	if errs.HasErrors() {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_QUERY_PARAMS", "Invalid query parameters", errs)
	}

	// Route through gRPC service mesh if available
	if ctrl.grpcClients != nil && ctrl.grpcClients.ProductClient != nil {
		req := &pb.ListProductsRequest{
			Page:       int32(filter.Page),
			Limit:      int32(filter.Limit),
			Search:     filter.Search,
			CategoryId: uint32(filter.CategoryID),
			Category:   filter.Category,
			Status:     filter.Status,
			Sort:       filter.Sort,
			Order:      filter.Order,
		}
		if filter.MinPrice != nil {
			req.MinPrice = filter.MinPrice
		}
		if filter.MaxPrice != nil {
			req.MaxPrice = filter.MaxPrice
		}

		grpcResp, err := ctrl.grpcClients.ProductClient.ListProducts(c.Context(), req)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}

		items := make([]fiber.Map, len(grpcResp.Products))
		for i, p := range grpcResp.Products {
			items[i] = fiber.Map{
				"id":              p.Id,
				"sku":             p.Sku,
				"name":            p.Name,
				"description":     p.Description,
				"category_id":     p.CategoryId,
				"category_name":   p.CategoryName,
				"price":           p.Price,
				"stock":           p.Stock,
				"reserved_stock":  p.ReservedStock,
				"available_stock": p.AvailableStock,
				"stock_status":    p.StockStatus,
				"status":          p.Status,
				"created_at":      p.CreatedAt,
			}
		}

		return response.Paginated(c, fiber.StatusOK, items, &response.Pagination{
			Page:       int(grpcResp.Page),
			Limit:      int(grpcResp.Limit),
			Total:      grpcResp.Total,
			TotalPages: int(grpcResp.TotalPages),
		})
	}

	// Direct service fallback
	products, pagination, err := ctrl.prodService.ListProducts(filter)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Paginated(c, fiber.StatusOK, products, pagination)
}

// GetByID handles GET /api/products/:id
// Employs Cache-Aside pattern backed by Redis (10m TTL) with X-Cache observability
func (ctrl *ProductController) GetByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || id == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	cacheKey := fmt.Sprintf("product:id:%d", id)

	// 1. Cache-Aside: Check Redis cache
	if ctrl.cacheService != nil {
		if cachedJSON, cacheErr := ctrl.cacheService.Get(c.Context(), cacheKey); cacheErr == nil && cachedJSON != "" {
			var cachedMap fiber.Map
			if err := json.Unmarshal([]byte(cachedJSON), &cachedMap); err == nil {
				c.Set("X-Cache", "HIT")
				return response.Success(c, cachedMap)
			}
		}
	}

	// Cache Miss: Mark header
	c.Set("X-Cache", "MISS")

	var result interface{}

	// 2. Route through gRPC service mesh if available
	if ctrl.grpcClients != nil && ctrl.grpcClients.ProductClient != nil {
		grpcResp, err := ctrl.grpcClients.ProductClient.GetProduct(c.Context(), &pb.GetProductRequest{Id: uint32(id)})
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.NotFound {
				return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			}
			return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		p := grpcResp.Product
		result = fiber.Map{
			"id":              p.Id,
			"sku":             p.Sku,
			"name":            p.Name,
			"description":     p.Description,
			"category_id":     p.CategoryId,
			"category_name":   p.CategoryName,
			"price":           p.Price,
			"stock":           p.Stock,
			"reserved_stock":  p.ReservedStock,
			"available_stock": p.AvailableStock,
			"stock_status":    p.StockStatus,
			"status":          p.Status,
			"created_at":      p.CreatedAt,
		}
	} else {
		// Direct service fallback
		product, err := ctrl.prodService.GetProductByID(uint(id))
		if err != nil {
			if errors.Is(err, repositories.ErrProductNotFound) {
				return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			}
			return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		result = product.ToResponse()
	}

	// 3. Cache the product result for 10 minutes
	if ctrl.cacheService != nil {
		_ = ctrl.cacheService.Set(c.Context(), cacheKey, result, 10*time.Minute)
	}

	return response.Success(c, result)
}

// Update handles PUT /api/products/:id and invalidates the product cache
func (ctrl *ProductController) Update(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || id == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	var req validators.UpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	var prodStatus models.ProductStatus
	if req.Status != "" {
		prodStatus = models.ProductStatus(req.Status)
	}

	product, err := ctrl.prodService.UpdateProduct(uint(id), req.Name, req.Description, req.CategoryID, req.Price, prodStatus)
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		if errors.Is(err, services.ErrCategoryNotFound) {
			return response.Error(c, fiber.StatusBadRequest, "CATEGORY_NOT_FOUND", "Specified category does not exist")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	// Invalidate Cache-Aside entry
	if ctrl.cacheService != nil {
		_ = ctrl.cacheService.Delete(c.Context(), fmt.Sprintf("product:id:%d", id))
	}

	return response.Success(c, product.ToResponse())
}

// Delete handles DELETE /api/products/:id (logical soft-deactivation) and invalidates cache
func (ctrl *ProductController) Delete(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || id == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	err = ctrl.prodService.DeactivateProduct(uint(id))
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	// Invalidate Cache-Aside entry
	if ctrl.cacheService != nil {
		_ = ctrl.cacheService.Delete(c.Context(), fmt.Sprintf("product:id:%d", id))
	}

	return response.Success(c, fiber.Map{
		"message": "Product deactivated successfully",
		"id":      id,
		"status":  models.ProductStatusInactive,
	})
}
