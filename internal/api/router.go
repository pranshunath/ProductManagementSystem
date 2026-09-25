package api

import (
	"errors"
	"time"

	"producthub/internal/controllers"
	"producthub/internal/services"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"gorm.io/gorm"
)

// RouterConfig holds dependencies required to construct the Fiber router
type RouterConfig struct {
	DB               *gorm.DB
	UserService      services.UserService
	CategoryService  services.CategoryService
	ProductService   services.ProductService
	InventoryService services.InventoryService
	OrderService     services.OrderService
	CartService      services.CartService
}

// SetupRouter initializes Fiber with global middleware and application routes
func SetupRouter(cfg RouterConfig) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "ProductHub — Product, Inventory & Order Management Platform",
		ServerHeader: "ProductHub-Gateway",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		// Centralized Error Handling
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "Internal server error"
			errCode := "INTERNAL_SERVER_ERROR"

			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
				message = e.Message
				switch code {
				case fiber.StatusBadRequest:
					errCode = "BAD_REQUEST"
				case fiber.StatusNotFound:
					errCode = "RESOURCE_NOT_FOUND"
				case fiber.StatusUnauthorized:
					errCode = "UNAUTHORIZED"
				case fiber.StatusForbidden:
					errCode = "FORBIDDEN"
				case fiber.StatusConflict:
					errCode = "CONFLICT"
				case fiber.StatusUnprocessableEntity:
					errCode = "UNPROCESSABLE_ENTITY"
				case fiber.StatusTooManyRequests:
					errCode = "RATE_LIMIT_EXCEEDED"
				default:
					errCode = "REQUEST_FAILED"
				}
			}

			return response.Error(c, code, errCode, message)
		},
	})

	// Global Middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	app.Use(requestid.New(requestid.Config{
		Header: "X-Request-ID",
	}))

	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} | ${method} ${path} | req_id=${locals:requestid}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, Idempotency-Key",
		AllowMethods:     "GET, POST, PUT, DELETE, PATCH, OPTIONS",
		AllowCredentials: false,
	}))

	// API Group
	apiGroup := app.Group("/api")

	// Health check
	healthController := controllers.NewHealthController(cfg.DB)
	apiGroup.Get("/health", healthController.Check)

	// Fallback 404 handler for unmatched routes
	app.Use(func(c *fiber.Ctx) error {
		return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "The requested endpoint does not exist")
	})

	return app
}
