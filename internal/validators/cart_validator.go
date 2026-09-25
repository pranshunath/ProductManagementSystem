package validators

// AddCartItemRequest defines payload for adding an item to shopping cart
type AddCartItemRequest struct {
	ProductID uint `json:"product_id"`
	Quantity  int  `json:"quantity"`
}

// Validate validates AddCartItemRequest fields
func (r *AddCartItemRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	if r.ProductID == 0 {
		errs = append(errs, FieldError{Field: "product_id", Message: "valid product_id is required"})
	}

	if r.Quantity <= 0 {
		errs = append(errs, FieldError{Field: "quantity", Message: "quantity must be at least 1"})
	}

	return errs
}

// UpdateCartItemRequest defines payload for updating an item's quantity in cart
type UpdateCartItemRequest struct {
	Quantity int `json:"quantity"`
}

// Validate validates UpdateCartItemRequest fields
func (r *UpdateCartItemRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	if r.Quantity < 0 {
		errs = append(errs, FieldError{Field: "quantity", Message: "quantity cannot be negative"})
	}

	return errs
}
