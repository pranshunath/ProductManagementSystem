package api

import (
	"errors"
	"time"

	"producthub/internal/controllers"
	"producthub/internal/grpc/clients"
	"producthub/internal/middleware"
	"producthub/internal/models"
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
	DB                 *gorm.DB
	JWTSecret          string
	JWTExpirationHours int
	UserService        services.UserService
	CategoryService    services.CategoryService
	ProductService     services.ProductService
	InventoryService   services.InventoryService
	OrderService       services.OrderService
	CartService        services.CartService
	GRPCClients        *clients.GRPCClients
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

	// Auth routes
	if cfg.UserService != nil {
		authController := controllers.NewAuthController(cfg.UserService, cfg.JWTSecret, cfg.JWTExpirationHours)
		auth := apiGroup.Group("/auth")
		auth.Post("/register", authController.Register)
		auth.Post("/login", authController.Login)
		auth.Get("/me", middleware.JWTAuth(cfg.JWTSecret), authController.Me)
	}

	// Category routes (Browsing is public; modification requires ADMIN)
	if cfg.CategoryService != nil {
		catController := controllers.NewCategoryController(cfg.CategoryService)
		categories := apiGroup.Group("/categories")
		categories.Get("/", catController.List)
		categories.Get("/:id", catController.GetByID)

		// Admin-protected operations
		adminCategories := categories.Group("", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRole(models.RoleAdmin))
		adminCategories.Post("/", catController.Create)
		adminCategories.Put("/:id", catController.Update)
		adminCategories.Delete("/:id", catController.Delete)
	}

	// Product routes (Browsing is public; modification requires ADMIN)
	if cfg.ProductService != nil {
		prodController := controllers.NewProductController(cfg.ProductService, cfg.GRPCClients)
		products := apiGroup.Group("/products")
		products.Get("/", prodController.List)
		products.Get("/:id", prodController.GetByID)

		// Admin-protected operations
		adminProducts := products.Group("", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRole(models.RoleAdmin))
		adminProducts.Post("/", prodController.Create)
		adminProducts.Put("/:id", prodController.Update)
		adminProducts.Delete("/:id", prodController.Delete)
	}

	// Inventory routes
	if cfg.InventoryService != nil && cfg.ProductService != nil {
		invController := controllers.NewInventoryController(cfg.InventoryService, cfg.ProductService)
		inventory := apiGroup.Group("/inventory")

		// Public stock queries
		inventory.Get("/products/:id/stock", invController.GetStock)
		inventory.Post("/check-stock", invController.CheckStock)

		// Admin-protected inventory operations
		adminInv := inventory.Group("", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRole(models.RoleAdmin))
		adminInv.Post("/restock", invController.Restock)
		adminInv.Get("/summary", invController.Summary)
		adminInv.Get("/transactions", invController.Transactions)
		adminInv.Get("/products/:id/transactions", invController.ProductTransactions)
	}

	// Cart routes (Protected: Authenticated Customer)
	if cfg.CartService != nil {
		cartController := controllers.NewCartController(cfg.CartService)
		cart := apiGroup.Group("/cart", middleware.JWTAuth(cfg.JWTSecret))
		cart.Get("/", cartController.GetCart)
		cart.Post("/items", cartController.AddItem)
		cart.Put("/items/:product_id", cartController.UpdateItem)
		cart.Delete("/items/:product_id", cartController.RemoveItem)
		cart.Delete("/", cartController.ClearCart)
	}

	// Order routes (Protected: Authenticated Customer)
	if cfg.OrderService != nil {
		orderController := controllers.NewOrderController(cfg.OrderService, cfg.CartService)
		orders := apiGroup.Group("/orders", middleware.JWTAuth(cfg.JWTSecret))
		orders.Post("/", orderController.Create)
		orders.Get("/", orderController.ListMyOrders)
		orders.Get("/:id", orderController.GetByID)
		orders.Post("/:id/cancel", orderController.Cancel)

		// Admin Order management
		adminOrders := apiGroup.Group("/admin/orders", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRole(models.RoleAdmin))
		adminOrders.Get("/", orderController.ListAll)
		adminOrders.Put("/:id/status", orderController.UpdateStatus)
	}

	// Fallback 404 handler for unmatched routes
	app.Use(func(c *fiber.Ctx) error {
		return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "The requested endpoint does not exist")
	})

	return app
}
