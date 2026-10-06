package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"producthub/internal/models"
)

func Seed(db *gorm.DB) error {
	// ------------------------------------------------------------
	// Categories
	// ------------------------------------------------------------

	categories := []models.Category{
		{
			Name:        "Electronics",
			Description: "Electronic devices and accessories",
		},
		{
			Name:        "Home & Office",
			Description: "Furniture and office equipment",
		},
		{
			Name:        "Accessories",
			Description: "Computer and mobile accessories",
		},
	}

	for _, category := range categories {
		var existing models.Category

		err := db.
			Where("name = ?", category.Name).
			First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&category).Error; err != nil {
				return fmt.Errorf(
					"failed to seed category %s: %w",
					category.Name,
					err,
				)
			}
		} else if err != nil {
			return fmt.Errorf(
				"failed to check category %s: %w",
				category.Name,
				err,
			)
		}
	}

	// Get category records so we can use their IDs.
	var elecCat models.Category
	var officeCat models.Category
	var accCat models.Category

	if err := db.Where("name = ?", "Electronics").First(&elecCat).Error; err != nil {
		return fmt.Errorf("failed to find Electronics category: %w", err)
	}

	if err := db.Where("name = ?", "Home & Office").First(&officeCat).Error; err != nil {
		return fmt.Errorf("failed to find Home & Office category: %w", err)
	}

	if err := db.Where("name = ?", "Accessories").First(&accCat).Error; err != nil {
		return fmt.Errorf("failed to find Accessories category: %w", err)
	}

	// ------------------------------------------------------------
	// Products
	// ------------------------------------------------------------

	products := []struct {
		Product      models.Product
		InitialStock int
	}{
		{
			Product: models.Product{
				SKU:         "SKU-MBP-16",
				Name:        "MacBook Pro 16-inch M3 Max",
				Description: "Top-tier workstation laptop with 36GB unified memory and 1TB SSD.",
				CategoryID:  elecCat.ID,
				Price:       3499.00,
				Stock:       15,
				Status:      models.ProductStatusActive,
				ImageURL:    "/images/products/macbook.jpg",
			},
			InitialStock: 15,
		},
		{
			Product: models.Product{
				SKU:         "SKU-MECH-KB",
				Name:        "Wireless Mechanical Keyboard RGB",
				Description: "Hot-swappable mechanical switches with low-latency Bluetooth and 2.4GHz wireless.",
				CategoryID:  elecCat.ID,
				Price:       129.99,
				Stock:       50,
				Status:      models.ProductStatusActive,
				ImageURL:    "/images/products/mechanical-keyboard.jpg",
			},
			InitialStock: 50,
		},
		{
			Product: models.Product{
				SKU:         "SKU-ANC-HEAD",
				Name:        "Studio ANC Wireless Headphones",
				Description: "Active noise cancellation with 40-hour battery life and spatial audio support.",
				CategoryID:  elecCat.ID,
				Price:       249.50,
				Stock:       8,
				Status:      models.ProductStatusActive,
				ImageURL:    "/images/products/headphones.jpg",
			},
			InitialStock: 8,
		},
		{
			Product: models.Product{
				SKU:         "SKU-ERGO-CHAIR",
				Name:        "Ergonomic Mesh Task Chair",
				Description: "Breathable mesh back with adjustable lumbar support and 4D armrests.",
				CategoryID:  officeCat.ID,
				Price:       450.00,
				Stock:       20,
				Status:      models.ProductStatusActive,
				ImageURL:    "/images/products/ergonomic-chair.jpg",
			},
			InitialStock: 20,
		},
		{
			Product: models.Product{
				SKU:         "SKU-USBC-HUB",
				Name:        "10-in-1 Aluminum USB-C Hub",
				Description: "Dual 4K HDMI, Gigabit Ethernet, 100W Power Delivery, and SD card reader.",
				CategoryID:  accCat.ID,
				Price:       69.99,
				Stock:       0,
				Status:      models.ProductStatusActive,
				ImageURL:    "/images/products/usb-c-hub.jpg",
			},
			InitialStock: 0,
		},
	}

	for _, item := range products {
		prod := item.Product

		// Check whether the product already exists.
		var existing models.Product

		err := db.
			Where("sku = ?", prod.SKU).
			First(&existing).Error

		if err == nil {
			// Product already exists.
			// Update its image URL so existing products
			// receive the newly added image path.
			if err := db.Model(&existing).Update(
				"image_url",
				prod.ImageURL,
			).Error; err != nil {
				return fmt.Errorf(
					"failed to update image URL for product %s: %w",
					prod.SKU,
					err,
				)
			}

			continue
		}

		if err != gorm.ErrRecordNotFound {
			return fmt.Errorf(
				"failed to check product %s: %w",
				prod.SKU,
				err,
			)
		}

		// Create product.
		if err := db.Create(&prod).Error; err != nil {
			return fmt.Errorf(
				"failed to seed product %s: %w",
				prod.SKU,
				err,
			)
		}

		// Create initial audit transaction.
		if item.InitialStock > 0 {
			tx := models.InventoryTransaction{
				ProductID:   prod.ID,
				Type:        models.TxTypeStockIn,
				Quantity:    item.InitialStock,
				ReferenceID: "INITIAL_SEED",
				Notes:       "Initial warehouse inventory setup",
				CreatedAt:   time.Now().UTC(),
			}

			if err := db.Create(&tx).Error; err != nil {
				log.Printf(
					"[WARNING] Failed to record initial transaction for %s: %v",
					prod.SKU,
					err,
				)
			}
		}
	}

	log.Println("[SEEDER] Initial catalog and inventory seeded successfully.")
		// ------------------------------------------------------------
	// Default Admin Account
	// ------------------------------------------------------------

	var adminUser models.User
	adminEmail := "admin@example.com"

	err := db.Where("email = ?", adminEmail).First(&adminUser).Error

	if err == gorm.ErrRecordNotFound {
		adminUser = models.User{
			Name:  "System Administrator",
			Email: adminEmail,
			Role:  models.RoleAdmin,
		}

		if err := adminUser.SetPassword("Admin@123"); err != nil {
			return fmt.Errorf("failed to set admin password: %w", err)
		}

		if err := db.Create(&adminUser).Error; err != nil {
			return fmt.Errorf("failed to seed admin user: %w", err)
		}

		log.Println("[SEEDER] Default admin account created.")
	} else if err != nil {
		return fmt.Errorf("failed to check admin user: %w", err)
	}

	return nil
}
