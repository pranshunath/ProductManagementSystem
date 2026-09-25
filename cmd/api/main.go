package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"producthub/internal/api"
	"producthub/internal/config"
	"producthub/internal/database"

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
	}

	// 3. Setup HTTP API Router (Fiber)
	app := api.SetupRouter(api.RouterConfig{
		DB: db,
	})

	// 4. Graceful Shutdown listener
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdownChan
		log.Println("[SHUTDOWN] Graceful shutdown signal received. Shutting down Fiber...")
		if err := app.Shutdown(); err != nil {
			log.Printf("[ERROR] Error during server shutdown: %v", err)
		}
	}()

	// 5. Start Server
	serverAddr := fmt.Sprintf(":%s", cfg.App.Port)
	log.Printf("[SERVER] Fiber API Gateway listening on http://localhost:%s\n", cfg.App.Port)
	log.Printf("[SERVER] Health endpoint: http://localhost:%s/api/health\n", cfg.App.Port)

	if err := app.Listen(serverAddr); err != nil {
		log.Fatalf("[FATAL] Fiber server failed to start: %v", err)
	}

	log.Println("[SHUTDOWN] ProductHub server stopped.")
}
