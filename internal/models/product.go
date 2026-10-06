package models

import (
	"time"

	"gorm.io/gorm"
)

// ProductStatus defines active state of a product
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "ACTIVE"
	ProductStatusInactive ProductStatus = "INACTIVE"
)

// Product represents a catalog item with inventory tracking
type Product struct {
	ID                uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	SKU               string         `gorm:"size:50;uniqueIndex;not null" json:"sku"`
	Name              string         `gorm:"size:200;not null;index" json:"name"`
	Description       string         `gorm:"type:text" json:"description"`
	CategoryID        uint           `gorm:"not null;index" json:"category_id"`
	Category          Category       `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"category,omitempty"`
	Price             float64        `gorm:"type:decimal(12,2);not null;default:0.00" json:"price"`
	ImageURL          string         `gorm:"size:500" json:"image_url"`
	Stock             int            `gorm:"not null;default:0" json:"stock"`
	ReservedStock     int            `gorm:"not null;default:0" json:"reserved_stock"`
	LowStockThreshold int            `gorm:"not null;default:10" json:"low_stock_threshold"`
	Status            ProductStatus  `gorm:"size:20;not null;default:'ACTIVE';index" json:"status"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// AvailableStock computes the purchasable quantity
func (p *Product) AvailableStock() int {
	available := p.Stock - p.ReservedStock
	if available < 0 {
		return 0
	}
	return available
}

// StockStatus returns an indicator enum for visual badges
func (p *Product) StockStatus() string {
	avail := p.AvailableStock()
	if avail <= 0 {
		return "OUT_OF_STOCK"
	}
	if avail < 10 {
		return "LOW_STOCK"
	}
	return "IN_STOCK"
}

// ProductResponse provides a formatted DTO for API responses
type ProductResponse struct {
	ID                uint          `json:"id"`
	SKU               string        `json:"sku"`
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	ImageURL          string        `json:"image_url"`
	CategoryID        uint          `json:"category_id"`
	CategoryName      string        `json:"category_name,omitempty"`
	Price             float64       `json:"price"`
	Stock             int           `json:"stock"`
	ReservedStock     int           `json:"reserved_stock"`
	LowStockThreshold int           `json:"low_stock_threshold"`
	AvailableStock    int           `json:"available_stock"`
	StockStatus       string        `json:"stock_status"`
	Status            ProductStatus `json:"status"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// ToResponse formats the Product model into ProductResponse
func (p *Product) ToResponse() ProductResponse {
	catName := ""
	if p.Category.Name != "" {
		catName = p.Category.Name
	}

	return ProductResponse{
		ID:                p.ID,
		SKU:               p.SKU,
		Name:              p.Name,
		Description:       p.Description,
		ImageURL:          p.ImageURL,
		CategoryID:        p.CategoryID,
		CategoryName:      catName,
		Price:             p.Price,
		Stock:             p.Stock,
		ReservedStock:     p.ReservedStock,
		LowStockThreshold: p.LowStockThreshold,
		AvailableStock:    p.AvailableStock(),
		StockStatus:       p.StockStatus(),
		Status:            p.Status,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}
