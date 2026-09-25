package validators

import (
	"fmt"
	"strings"
)

// FieldError represents a specific input validation failure
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a slice of FieldError
type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	var msgs []string
	for _, err := range v {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, ", ")
}

// HasErrors returns true if there are any validation errors
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}
