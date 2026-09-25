package database

import (
	"fmt"
	"log"
	"time"

	"producthub/internal/models"

	"gorm.io/gorm"
)

// Seed populates the database with initial baseline data if tables are empty
func Seed(db *gorm.DB) error {
	log.Println("[SEEDER] Checking database baseline records...")

	// 1. Seed Users (Admin & Customer)
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		log.Println("[SEEDER] Seeding initial users...")

		admin := models.User{
			Name:  "System Administrator",
			Email: "admin@producthub.com",
			Role:  models.RoleAdmin,
		}
		if err := admin.SetPassword("Admin@123"); err != nil {
			return fmt.Errorf("failed to hash admin password: %w", err)
		}

		customer := models.User{
			Name:  "John Doe",
			Email: "customer@producthub.com",
			Role:  models.RoleCustomer,
		}
		if err := customer.SetPassword("Customer@123"); err != nil {
			return fmt.Errorf("failed to hash customer password: %w", err)
		}

		if err := db.Create(&admin).Error; err != nil {
			return fmt.Errorf("failed to seed admin: %w", err)
		}
		if err := db.Create(&customer).Error; err != nil {
			return fmt.Errorf("failed to seed customer: %w", err)
		}
		log.Println("[SEEDER] Admin (admin@producthub.com) and Customer (customer@producthub.com) seeded.")
	}

	// 2. Seed Categories
	var categoryCount int64
	db.Model(&models.Category{}).Count(&categoryCount)
	if categoryCount == 0 {
		log.Println("[SEEDER] Seeding initial categories...")
		categories := []models.Category{
			{Name: "Electronics", Description: "Laptops, phones, audio devices, and computer peripherals"},
			{Name: "Home & Office", Description: "Ergonomic furniture, lighting, and workspace essentials"},
			{Name: "Books & Learning", Description: "Technical books, notebooks, and reference materials"},
			{Name: "Accessories", Description: "Cables, adapters, bags, and everyday carry gear"},
		}
		for i := range categories {
			if err := db.Create(&categories[i]).Error; err != nil {
				return fmt.Errorf("failed to seed category %s: %w", categories[i].Name, err)
			}
		}
		log.Println("[SEEDER] Categories seeded.")
	}

	// 3. Seed Products
	var productCount int64
	db.Model(&models.Product{}).Count(&productCount)
	if productCount == 0 {
		log.Println("[SEEDER] Seeding initial products with inventory transactions...")

		var elecCat, officeCat, accCat models.Category
		db.Where("name = ?", "Electronics").First(&elecCat)
		db.Where("name = ?", "Home & Office").First(&officeCat)
		db.Where("name = ?", "Accessories").First(&accCat)

		products := []struct {
			Product     models.Product
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
					Stock:       8, // Low stock on purpose
					Status:      models.ProductStatusActive,
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
					Stock:       0, // Out of stock on purpose
					Status:      models.ProductStatusActive,
				},
				InitialStock: 0,
			},
		}

		for _, item := range products {
			prod := item.Product
			if err := db.Create(&prod).Error; err != nil {
				return fmt.Errorf("failed to seed product %s: %w", prod.SKU, err)
			}

			// Create initial audit transaction
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
					log.Printf("[WARNING] Failed to record initial transaction for %s: %v", prod.SKU, err)
				}
			}
		}
		log.Println("[SEEDER] Initial catalog and inventory seeded successfully.")
	}

	return nil
}
