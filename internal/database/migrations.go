package database

import (
	"log"

	"producthub/internal/models"

	"gorm.io/gorm"
)

// AutoMigrate runs schema auto-migrations for all database entities
func AutoMigrate(db *gorm.DB) error {
	log.Println("[MIGRATION] Running automated database schema migrations...")

	err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
		&models.InventoryTransaction{},
		&models.Cart{},
		&models.CartItem{},
	)
	if err != nil {
		return err
	}

	log.Println("[MIGRATION] Database schema migration completed successfully.")
	return nil
}
