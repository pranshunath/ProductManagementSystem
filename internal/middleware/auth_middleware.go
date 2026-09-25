package middleware

import (
	"errors"
	"strings"

	"producthub/internal/models"
	"producthub/pkg/jwt"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

var (
	ErrUnauthenticated = errors.New("user is not authenticated")
)

// JWTAuth middleware verifies the JWT Bearer token and injects claims into context
func JWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing Authorization header")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Malformed Authorization header; expected 'Bearer <token>'")
		}

		tokenStr := strings.TrimSpace(parts[1])
		claims, err := jwt.ValidateToken(tokenStr, secret)
		if err != nil {
			if errors.Is(err, jwt.ErrExpiredToken) {
				return response.Error(c, fiber.StatusUnauthorized, "TOKEN_EXPIRED", "Authentication token has expired")
			}
			return response.Error(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Invalid authentication token")
		}

		// Inject user metadata into Fiber context locals
		c.Locals("userID", claims.UserID)
		c.Locals("userEmail", claims.Email)
		c.Locals("userRole", claims.Role)

		return c.Next()
	}
}

// RequireRole middleware ensures the authenticated user possesses at least one of the allowed roles
func RequireRole(allowedRoles ...models.UserRole) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleVal := c.Locals("userRole")
		if roleVal == nil {
			return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		}

		userRole, ok := roleVal.(models.UserRole)
		if !ok {
			return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "Invalid user role in token")
		}

		for _, allowed := range allowedRoles {
			if userRole == allowed {
				return c.Next()
			}
		}

		return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "Access denied: insufficient permissions")
	}
}

// GetAuthenticatedUserID extracts the verified user ID from Fiber context
func GetAuthenticatedUserID(c *fiber.Ctx) (uint, error) {
	val := c.Locals("userID")
	if val == nil {
		return 0, ErrUnauthenticated
	}
	id, ok := val.(uint)
	if !ok {
		return 0, ErrUnauthenticated
	}
	return id, nil
}

// GetAuthenticatedUserRole extracts the user role from context
func GetAuthenticatedUserRole(c *fiber.Ctx) models.UserRole {
	val := c.Locals("userRole")
	if val == nil {
		return ""
	}
	role, ok := val.(models.UserRole)
	if !ok {
		return ""
	}
	return role
}

// IsAdmin returns true if the authenticated user has an ADMIN role
func IsAdmin(c *fiber.Ctx) bool {
	return GetAuthenticatedUserRole(c) == models.RoleAdmin
}
