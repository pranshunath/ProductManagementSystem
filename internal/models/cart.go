package models

import (
	"time"

	"gorm.io/gorm"
)

// Cart represents a user's active shopping cart
type Cart struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user,omitempty"`
	Items     []CartItem     `gorm:"foreignKey:CartID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// CartItem represents an item added to a cart
type CartItem struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CartID    uint      `gorm:"not null;uniqueIndex:idx_cart_product" json:"cart_id"`
	ProductID uint      `gorm:"not null;uniqueIndex:idx_cart_product;index" json:"product_id"`
	Product   Product   `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"product,omitempty"`
	Quantity  int       `gorm:"not null;default:1" json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
