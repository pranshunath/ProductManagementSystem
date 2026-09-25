package controllers

import (
	"errors"

	"producthub/internal/middleware"
	"producthub/internal/models"
	"producthub/internal/services"
	"producthub/internal/validators"
	"producthub/pkg/jwt"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// AuthController handles registration, authentication, and user profile endpoints
type AuthController struct {
	userService         services.UserService
	jwtSecret           string
	jwtExpirationHours  int
}

// NewAuthController creates a new instance of AuthController
func NewAuthController(userService services.UserService, jwtSecret string, jwtExpirationHours int) *AuthController {
	return &AuthController{
		userService:        userService,
		jwtSecret:          jwtSecret,
		jwtExpirationHours: jwtExpirationHours,
	}
}

// Register handles POST /api/auth/register
func (ctrl *AuthController) Register(c *fiber.Ctx) error {
	var req validators.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	role := models.RoleCustomer
	if req.Role != "" {
		role = models.UserRole(req.Role)
	}

	user, err := ctrl.userService.Register(req.Name, req.Email, req.Password, role)
	if err != nil {
		if errors.Is(err, services.ErrEmailAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "EMAIL_EXISTS", err.Error())
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	// Issue JWT token upon successful registration
	token, err := jwt.GenerateToken(user.ID, user.Email, user.Role, ctrl.jwtSecret, ctrl.jwtExpirationHours)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate authentication token")
	}

	return response.Created(c, fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":         user.ID,
			"name":       user.Name,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		},
	})
}

// Login handles POST /api/auth/login
func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req validators.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
	}

	if errs := req.Validate(); errs.HasErrors() {
		return response.Error(c, fiber.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed", errs)
	}

	user, err := ctrl.userService.Authenticate(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			return response.Error(c, fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	token, err := jwt.GenerateToken(user.ID, user.Email, user.Role, ctrl.jwtSecret, ctrl.jwtExpirationHours)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate authentication token")
	}

	return response.Success(c, fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":         user.ID,
			"name":       user.Name,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		},
	})
}

// Me handles GET /api/auth/me (Protected route)
func (ctrl *AuthController) Me(c *fiber.Ctx) error {
	userID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
	}

	user, err := ctrl.userService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			return response.Error(c, fiber.StatusNotFound, "USER_NOT_FOUND", "User profile not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}

	return response.Success(c, fiber.Map{
		"id":         user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"role":       user.Role,
		"created_at": user.CreatedAt,
	})
}
