package controllers

import (
	"errors"
	"strconv"

	"producthub/internal/services"
	"producthub/internal/validators"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// CategoryController handles HTTP requests for product categories
type CategoryController struct {
	catService services.CategoryService
}

// NewCategoryController creates a new instance of CategoryController
func NewCategoryController(catService services.CategoryService) *CategoryController {
	return &CategoryController{catService: catService}
}

// Create handles POST /api/categories
func (ctrl *CategoryController) Create(c *fiber.Ctx) error {
	var req validators.CategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	cat, err := ctrl.catService.CreateCategory(req.Name, req.Description)
	if err != nil {
		if errors.Is(err, services.ErrCategoryNameUsed) {
			return response.Error(c, fiber.StatusConflict, "CATEGORY_EXISTS", err.Error())
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Created(c, cat)
}

// List handles GET /api/categories
func (ctrl *CategoryController) List(c *fiber.Ctx) error {
	categories, err := ctrl.catService.ListCategories()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
	return response.Success(c, categories)
}

// GetByID handles GET /api/categories/:id
func (ctrl *CategoryController) GetByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || id == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Category ID must be a positive integer")
	}

	cat, err := ctrl.catService.GetCategory(uint(id))
	if err != nil {
		if errors.Is(err, services.ErrCategoryNotFound) {
			return response.Error(c, fiber.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, cat)
}

// Update handles PUT /api/categories/:id
func (ctrl *CategoryController) Update(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || id == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Category ID must be a positive integer")
	}

	var req validators.CategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	cat, err := ctrl.catService.UpdateCategory(uint(id), req.Name, req.Description)
	if err != nil {
		if errors.Is(err, services.ErrCategoryNotFound) {
			return response.Error(c, fiber.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		}
		if errors.Is(err, services.ErrCategoryNameUsed) {
			return response.Error(c, fiber.StatusConflict, "CATEGORY_EXISTS", err.Error())
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, cat)
}

// Delete handles DELETE /api/categories/:id
func (ctrl *CategoryController) Delete(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil || id == 0 {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Category ID must be a positive integer")
	}

	err = ctrl.catService.DeleteCategory(uint(id))
	if err != nil {
		if errors.Is(err, services.ErrCategoryNotFound) {
			return response.Error(c, fiber.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Category deleted successfully",
		"id":      id,
	})
}
