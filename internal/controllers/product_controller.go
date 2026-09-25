package controllers

import (
	"errors"
	"strconv"

	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/internal/validators"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ProductController handles HTTP requests for product catalog
type ProductController struct {
	prodService services.ProductService
}

// NewProductController creates a new instance of ProductController
func NewProductController(prodService services.ProductService) *ProductController {
	return &ProductController{prodService: prodService}
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
func (ctrl *ProductController) List(c *fiber.Ctx) error {
	filter, errs := validators.ParseProductFilter(c)
	if errs.HasErrors() {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_QUERY_PARAMS", "Invalid query parameters", errs)
	}

	products, pagination, err := ctrl.prodService.ListProducts(filter)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Paginated(c, fiber.StatusOK, products, pagination)
}

// GetByID handles GET /api/products/:id
func (ctrl *ProductController) GetByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || id == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Product ID must be a positive integer")
	}

	product, err := ctrl.prodService.GetProductByID(uint(id))
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, product.ToResponse())
}

// Update handles PUT /api/products/:id
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

	var status models.ProductStatus
	if req.Status != "" {
		status = models.ProductStatus(req.Status)
	}

	product, err := ctrl.prodService.UpdateProduct(uint(id), req.Name, req.Description, req.CategoryID, req.Price, status)
	if err != nil {
		if errors.Is(err, repositories.ErrProductNotFound) {
			return response.Error(c, fiber.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		}
		if errors.Is(err, services.ErrCategoryNotFound) {
			return response.Error(c, fiber.StatusBadRequest, "CATEGORY_NOT_FOUND", "Specified category does not exist")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, product.ToResponse())
}

// Delete handles DELETE /api/products/:id (logical soft-deactivation)
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

	return response.Success(c, fiber.Map{
		"message": "Product deactivated successfully",
		"id":      id,
		"status":  models.ProductStatusInactive,
	})
}
