package validators

import (
	"regexp"
	"strings"

	"producthub/internal/models"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// RegisterRequest defines payload for user registration
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Validate validates RegisterRequest fields
func (r *RegisterRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	cleanName := strings.TrimSpace(r.Name)
	if cleanName == "" {
		errs = append(errs, FieldError{Field: "name", Message: "name is required"})
	} else if len(cleanName) < 2 || len(cleanName) > 100 {
		errs = append(errs, FieldError{Field: "name", Message: "name must be between 2 and 100 characters"})
	}

	cleanEmail := strings.TrimSpace(r.Email)
	if cleanEmail == "" {
		errs = append(errs, FieldError{Field: "email", Message: "email is required"})
	} else if !emailRegex.MatchString(cleanEmail) {
		errs = append(errs, FieldError{Field: "email", Message: "valid email address is required"})
	}

	if len(r.Password) < 6 {
		errs = append(errs, FieldError{Field: "password", Message: "password must be at least 6 characters long"})
	}

	if r.Role != "" {
		upperRole := strings.ToUpper(r.Role)
		if upperRole != string(models.RoleCustomer) && upperRole != string(models.RoleAdmin) {
			errs = append(errs, FieldError{Field: "role", Message: "role must be either CUSTOMER or ADMIN"})
		}
	}

	return errs
}

// LoginRequest defines payload for user authentication
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validate validates LoginRequest fields
func (r *LoginRequest) Validate() ValidationErrors {
	var errs ValidationErrors

	cleanEmail := strings.TrimSpace(r.Email)
	if cleanEmail == "" {
		errs = append(errs, FieldError{Field: "email", Message: "email is required"})
	} else if !emailRegex.MatchString(cleanEmail) {
		errs = append(errs, FieldError{Field: "email", Message: "valid email address is required"})
	}

	if strings.TrimSpace(r.Password) == "" {
		errs = append(errs, FieldError{Field: "password", Message: "password is required"})
	}

	return errs
}
