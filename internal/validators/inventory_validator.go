package validators

import (
	"strings"
)

// RestockRequest defines payload for restocking product inventory
type RestockRequest struct {
	ProductID uint   `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Notes     string `json:"notes"`
}

// Validate validates RestockRequest fields
func (r *RestockRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	if r.ProductID == 0 {
		errs = append(errs, FieldError{Field: "product_id", Message: "valid product_id is required"})
	}

	if r.Quantity <= 0 {
		errs = append(errs, FieldError{Field: "quantity", Message: "quantity must be greater than zero"})
	}

	if len(strings.TrimSpace(r.Notes)) > 255 {
		errs = append(errs, FieldError{Field: "notes", Message: "notes cannot exceed 255 characters"})
	}

	return errs
}

// CheckStockBatchRequest defines payload for checking stock of a product
type CheckStockBatchRequest struct {
	ProductID uint `json:"product_id"`
	Quantity  int  `json:"quantity"`
}

// Validate validates CheckStockBatchRequest fields
func (r *CheckStockBatchRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	if r.ProductID == 0 {
		errs = append(errs, FieldError{Field: "product_id", Message: "valid product_id is required"})
	}

	if r.Quantity <= 0 {
		errs = append(errs, FieldError{Field: "quantity", Message: "quantity must be greater than zero"})
	}

	return errs
}
