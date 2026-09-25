package validators

import (
	"strconv"
	"strings"

	"producthub/internal/models"
	"producthub/internal/repositories"

	"github.com/gofiber/fiber/v2"
)

// CreateProductRequest defines payload for registering a new product
type CreateProductRequest struct {
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CategoryID  uint    `json:"category_id"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
}

// Validate validates CreateProductRequest fields
func (r *CreateProductRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	cleanSKU := strings.TrimSpace(r.SKU)
	if cleanSKU == "" {
		errs = append(errs, FieldError{Field: "sku", Message: "SKU is required"})
	} else if len(cleanSKU) < 3 || len(cleanSKU) > 50 {
		errs = append(errs, FieldError{Field: "sku", Message: "SKU must be between 3 and 50 characters"})
	}

	cleanName := strings.TrimSpace(r.Name)
	if cleanName == "" {
		errs = append(errs, FieldError{Field: "name", Message: "product name is required"})
	} else if len(cleanName) < 2 || len(cleanName) > 200 {
		errs = append(errs, FieldError{Field: "name", Message: "product name must be between 2 and 200 characters"})
	}

	if r.CategoryID == 0 {
		errs = append(errs, FieldError{Field: "category_id", Message: "valid category_id is required"})
	}

	if r.Price <= 0 {
		errs = append(errs, FieldError{Field: "price", Message: "price must be greater than 0"})
	}

	if r.Stock < 0 {
		errs = append(errs, FieldError{Field: "stock", Message: "stock cannot be negative"})
	}

	return errs
}

// UpdateProductRequest defines payload for updating an existing product
type UpdateProductRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CategoryID  uint    `json:"category_id"`
	Price       float64 `json:"price"`
	Status      string  `json:"status"`
}

// Validate validates UpdateProductRequest fields
func (r *UpdateProductRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	if r.Name != "" && (len(strings.TrimSpace(r.Name)) < 2 || len(strings.TrimSpace(r.Name)) > 200) {
		errs = append(errs, FieldError{Field: "name", Message: "product name must be between 2 and 200 characters"})
	}

	if r.Price < 0 {
		errs = append(errs, FieldError{Field: "price", Message: "price cannot be negative"})
	}

	if r.Status != "" {
		upperStatus := strings.ToUpper(r.Status)
		if upperStatus != string(models.ProductStatusActive) && upperStatus != string(models.ProductStatusInactive) {
			errs = append(errs, FieldError{Field: "status", Message: "status must be either ACTIVE or INACTIVE"})
		}
	}

	return errs
}

// Allowed sort columns to strictly guard against SQL injection
var allowedSortColumns = map[string]bool{
	"price":      true,
	"name":       true,
	"created_at": true,
	"stock":      true,
	"id":         true,
}

// ParseProductFilter extracts, sanitizes, and validates query parameters for listing products
func ParseProductFilter(c *fiber.Ctx) (repositories.ProductFilter, ValidationErrors) {
	var errs ValidationErrors
	filter := repositories.ProductFilter{
		Page:     1,
		Limit:    10,
		Sort:     "id",
		Order:    "desc",
		Status:   "", // defaults to active in service unless specified
	}

	// 1. Page
	if pageStr := c.Query("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			errs = append(errs, FieldError{Field: "page", Message: "page must be a positive integer"})
		} else {
			filter.Page = page
		}
	}

	// 2. Limit
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			errs = append(errs, FieldError{Field: "limit", Message: "limit must be an integer between 1 and 100"})
		} else {
			filter.Limit = limit
		}
	}

	// 3. Search
	filter.Search = strings.TrimSpace(c.Query("search"))

	// 4. Category (name or ID)
	filter.Category = strings.TrimSpace(c.Query("category"))
	if catIDStr := c.Query("category_id"); catIDStr != "" {
		catID, err := strconv.ParseUint(catIDStr, 10, 32)
		if err != nil || catID == 0 {
			errs = append(errs, FieldError{Field: "category_id", Message: "category_id must be a positive integer"})
		} else {
			filter.CategoryID = uint(catID)
		}
	}

	// 5. Min Price
	if minStr := c.Query("min_price"); minStr != "" {
		minP, err := strconv.ParseFloat(minStr, 64)
		if err != nil || minP < 0 {
			errs = append(errs, FieldError{Field: "min_price", Message: "min_price must be a non-negative number"})
		} else {
			filter.MinPrice = &minP
		}
	}

	// 6. Max Price
	if maxStr := c.Query("max_price"); maxStr != "" {
		maxP, err := strconv.ParseFloat(maxStr, 64)
		if err != nil || maxP < 0 {
			errs = append(errs, FieldError{Field: "max_price", Message: "max_price must be a non-negative number"})
		} else {
			filter.MaxPrice = &maxP
		}
	}

	// Validate price range if both min and max are provided
	if filter.MinPrice != nil && filter.MaxPrice != nil && *filter.MinPrice > *filter.MaxPrice {
		errs = append(errs, FieldError{Field: "price_range", Message: "min_price cannot be greater than max_price"})
	}

	// 7. Status
	if statusStr := c.Query("status"); statusStr != "" {
		upperStatus := strings.ToUpper(statusStr)
		if upperStatus != string(models.ProductStatusActive) && upperStatus != string(models.ProductStatusInactive) {
			errs = append(errs, FieldError{Field: "status", Message: "status must be either ACTIVE or INACTIVE"})
		} else {
			filter.Status = upperStatus
		}
	}

	// 8. Sorting (whitelisted)
	if sortStr := strings.ToLower(strings.TrimSpace(c.Query("sort"))); sortStr != "" {
		if !allowedSortColumns[sortStr] {
			errs = append(errs, FieldError{Field: "sort", Message: "sort must be one of: price, name, created_at, stock, id"})
		} else {
			filter.Sort = sortStr
		}
	}

	// 9. Order (whitelisted)
	if orderStr := strings.ToLower(strings.TrimSpace(c.Query("order"))); orderStr != "" {
		if orderStr != "asc" && orderStr != "desc" {
			errs = append(errs, FieldError{Field: "order", Message: "order must be either asc or desc"})
		} else {
			filter.Order = orderStr
		}
	}

	return filter, errs
}
