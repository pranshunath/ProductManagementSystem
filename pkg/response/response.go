package response

import (
	"github.com/gofiber/fiber/v2"
)

// Response represents a standard JSON API response structure
type Response struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Error      *APIError   `json:"error,omitempty"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// APIError represents structured error details
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// JSON sends a standard JSON response with HTTP status code
func JSON(c *fiber.Ctx, statusCode int, data interface{}) error {
	return c.Status(statusCode).JSON(Response{
		Success: true,
		Data:    data,
	})
}

// Success sends a 200 OK standard JSON response
func Success(c *fiber.Ctx, data interface{}) error {
	return JSON(c, fiber.StatusOK, data)
}

// Created sends a 201 Created standard JSON response
func Created(c *fiber.Ctx, data interface{}) error {
	return JSON(c, fiber.StatusCreated, data)
}

// Paginated sends a standard paginated JSON response
func Paginated(c *fiber.Ctx, statusCode int, data interface{}, pagination *Pagination) error {
	return c.Status(statusCode).JSON(Response{
		Success:    true,
		Data:       data,
		Pagination: pagination,
	})
}

// Error sends a standardized JSON error response
func Error(c *fiber.Ctx, statusCode int, code string, message string, details ...interface{}) error {
	var errDetails interface{}
	if len(details) > 0 {
		errDetails = details[0]
	}

	return c.Status(statusCode).JSON(Response{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: errDetails,
		},
	})
}
