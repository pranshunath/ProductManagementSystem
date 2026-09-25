package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"producthub/internal/api"
	"producthub/internal/cache"
	"producthub/internal/config"
	"producthub/internal/database"
	"producthub/internal/grpc/clients"
	"producthub/internal/grpc/servers"
	"producthub/internal/repositories"
	"producthub/internal/services"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

func main() {
	log.Println("==================================================")
	log.Println(" Starting ProductHub — Product, Inventory & Order Platform")
	log.Println("==================================================")

	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Failed to load configuration: %v", err)
	}

	// 2. Connect to MySQL Database
	var db *gorm.DB
	var userRepo repositories.UserRepository
	var catRepo repositories.CategoryRepository
	var prodRepo repositories.ProductRepository
	var invRepo repositories.InventoryRepository
	var orderRepo repositories.OrderRepository
	var cartRepo repositories.CartRepository

	var userService services.UserService
	var catService services.CategoryService
	var prodService services.ProductService
	var invService services.InventoryService
	var orderService services.OrderService
	var cartService services.CartService

	db, err = database.ConnectMySQL(&cfg.MySQL)
	if err != nil {
		log.Printf("[WARNING] MySQL connection failed: %v", err)
		log.Println("[WARNING] Starting application in DEGRADED mode (verify MYSQL_USER, MYSQL_PASSWORD in .env)")
	} else {
		log.Println("[INFO] MySQL connection established and pool configured.")
		defer func() {
			if err := database.CloseMySQL(db); err != nil {
				log.Printf("[ERROR] Error closing MySQL connection: %v", err)
			}
		}()

		// Run automated schema migrations
		if err := database.AutoMigrate(db); err != nil {
			log.Fatalf("[FATAL] Database auto-migration failed: %v", err)
		}

		// Seed baseline data (Admin, Categories, Products)
		if err := database.Seed(db); err != nil {
			log.Printf("[WARNING] Database seeding warning: %v", err)
		}

		// Initialize Repositories
		userRepo = repositories.NewUserRepository(db)
		catRepo = repositories.NewCategoryRepository(db)
		prodRepo = repositories.NewProductRepository(db)
		invRepo = repositories.NewInventoryRepository(db)
		orderRepo = repositories.NewOrderRepository(db)
		cartRepo = repositories.NewCartRepository(db)

		// Initialize Services (Domain Business Logic)
		userService = services.NewUserService(userRepo)
		catService = services.NewCategoryService(catRepo)
		prodService = services.NewProductService(prodRepo, catRepo, invRepo)
		invService = services.NewInventoryService(invRepo, prodRepo)
		orderService = services.NewOrderService(orderRepo, invRepo, prodRepo, userRepo)
		cartService = services.NewCartService(cartRepo, prodRepo)
	}

	// 3. Initialize Redis Cache Service (falls back to memory cache if Redis is offline)
	cacheService := cache.NewRedisCache(&cfg.Redis)
	defer func() {
		if err := cacheService.Close(); err != nil {
			log.Printf("[ERROR] Error closing cache connection: %v", err)
		}
	}()

	// 4. Initialize and Start internal gRPC Server
	var grpcServer *grpc.Server
	var grpcClients *clients.GRPCClients

	grpcServer = servers.InitGRPCServer(prodService, invService, orderService, invRepo)
	grpcLis, err := servers.StartGRPCServer(cfg.GRPC.Port, grpcServer)
	if err != nil {
		log.Printf("[WARNING] Could not start gRPC server on port %s: %v", cfg.GRPC.Port, err)
	} else {
		defer grpcLis.Close()
		clientsConn, err := clients.NewGRPCClients("127.0.0.1:" + cfg.GRPC.Port)
		if err != nil {
			log.Printf("[WARNING] Could not connect to internal gRPC server: %v", err)
		} else {
			grpcClients = clientsConn
			defer grpcClients.Close()
		}
	}

	// 5. Setup HTTP API Router (Fiber Gateway)
	app := api.SetupRouter(api.RouterConfig{
		DB:                 db,
		JWTSecret:          cfg.JWT.Secret,
		JWTExpirationHours: cfg.JWT.ExpirationHours,
		UserService:        userService,
		CategoryService:    catService,
		ProductService:     prodService,
		InventoryService:   invService,
		OrderService:       orderService,
		CartService:        cartService,
		GRPCClients:        grpcClients,
		CacheService:       cacheService,
	})

	// 5. Graceful Shutdown listener
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdownChan
		log.Println("[SHUTDOWN] Graceful shutdown signal received. Shutting down Fiber and gRPC...")
		if grpcServer != nil {
			grpcServer.GracefulStop()
		}
		if err := app.Shutdown(); err != nil {
			log.Printf("[ERROR] Error during server shutdown: %v", err)
		}
	}()

	// 6. Start HTTP API Gateway Server
	serverAddr := fmt.Sprintf(":%s", cfg.App.Port)
	log.Printf("[SERVER] Fiber API Gateway listening on http://localhost:%s\n", cfg.App.Port)
	log.Printf("[SERVER] Health endpoint: http://localhost:%s/api/health\n", cfg.App.Port)

	if err := app.Listen(serverAddr); err != nil {
		log.Fatalf("[FATAL] Fiber server failed to start: %v", err)
	}

	log.Println("[SHUTDOWN] ProductHub server stopped.")
}
