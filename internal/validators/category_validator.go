package validators

import (
	"strings"
)

// CategoryRequest defines payload for creating or updating a category
type CategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Validate validates CategoryRequest fields
func (r *CategoryRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	cleanName := strings.TrimSpace(r.Name)
	if cleanName == "" {
		errs = append(errs, FieldError{
			Field:   "name",
			Message: "category name is required",
		})
	} else if len(cleanName) < 2 || len(cleanName) > 100 {
		errs = append(errs, FieldError{
			Field:   "name",
			Message: "category name must be between 2 and 100 characters",
		})
	}

	return errs
}
