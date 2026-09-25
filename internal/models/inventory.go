package models

import (
	"time"
)

// InventoryTransactionType defines nature of stock movement
type InventoryTransactionType string

const (
	TxTypeStockIn  InventoryTransactionType = "STOCK_IN"
	TxTypeStockOut InventoryTransactionType = "STOCK_OUT"
	TxTypeReserved InventoryTransactionType = "RESERVED"
	TxTypeReleased InventoryTransactionType = "RELEASED"
)

// InventoryTransaction provides an immutable audit trail for every stock change
type InventoryTransaction struct {
	ID          uint                     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProductID   uint                     `gorm:"not null;index" json:"product_id"`
	Product     Product                  `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"product,omitempty"`
	Type        InventoryTransactionType `gorm:"size:20;not null;index" json:"type"`
	Quantity    int                      `gorm:"not null" json:"quantity"`
	ReferenceID string                   `gorm:"size:100;index" json:"reference_id"` // E.g., Order ID, PO number, or manual adjustment
	Notes       string                   `gorm:"size:255" json:"notes,omitempty"`
	CreatedAt   time.Time                `gorm:"not null;index" json:"created_at"`
}
