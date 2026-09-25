package validators

import (
	"strings"

	"producthub/internal/models"
	"producthub/internal/services"
)

// CreateOrderRequest defines payload for placing an order
type CreateOrderRequest struct {
	Items          []services.OrderItemInput `json:"items"`
	IdempotencyKey string                    `json:"idempotency_key"`
}

// Validate validates CreateOrderRequest fields
func (r *CreateOrderRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	for i, item := range r.Items {
		if item.ProductID == 0 {
			errs = append(errs, FieldError{Field: "product_id", Message: "valid product_id is required"})
		}
		if item.Quantity <= 0 {
			errs = append(errs, FieldError{Field: "quantity", Message: "item quantity must be greater than zero"})
		}
		_ = i
	}

	return errs
}

// UpdateOrderStatusRequest defines payload for updating an order's status
type UpdateOrderStatusRequest struct {
	Status string `json:"status"`
}

// Validate validates UpdateOrderStatusRequest fields
func (r *UpdateOrderStatusRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	cleanStatus := strings.ToUpper(strings.TrimSpace(r.Status))
	if cleanStatus == "" {
		errs = append(errs, FieldError{Field: "status", Message: "status is required"})
		return errs
	}

	validStatuses := map[models.OrderStatus]bool{
		models.OrderStatusPending:    true,
		models.OrderStatusConfirmed:  true,
		models.OrderStatusProcessing: true,
		models.OrderStatusShipped:    true,
		models.OrderStatusDelivered:  true,
		models.OrderStatusCancelled:  true,
	}

	if !validStatuses[models.OrderStatus(cleanStatus)] {
		errs = append(errs, FieldError{
			Field:   "status",
			Message: "invalid status; must be one of: PENDING, CONFIRMED, PROCESSING, SHIPPED, DELIVERED, CANCELLED",
		})
	}

	return errs
}
