package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// OrderStatus defines valid stages in order lifecycle
type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "PENDING"
	OrderStatusConfirmed  OrderStatus = "CONFIRMED"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusShipped    OrderStatus = "SHIPPED"
	OrderStatusDelivered  OrderStatus = "DELIVERED"
	OrderStatusCancelled  OrderStatus = "CANCELLED"
)

// Order represents a customer purchase
type Order struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	User        User           `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"user,omitempty"`
	Status      OrderStatus    `gorm:"size:30;not null;default:'PENDING';index" json:"status"`
	TotalAmount float64        `gorm:"type:decimal(12,2);not null;default:0.00" json:"total_amount"`
	Items       []OrderItem    `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// OrderItem represents a line item in an order
type OrderItem struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID   uint      `gorm:"not null;index" json:"order_id"`
	ProductID uint      `gorm:"not null;index" json:"product_id"`
	Product   Product   `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"product,omitempty"`
	Quantity  int       `gorm:"not null;default:1" json:"quantity"`
	UnitPrice float64   `gorm:"type:decimal(12,2);not null;default:0.00" json:"unit_price"`
	Subtotal  float64   `gorm:"type:decimal(12,2);not null;default:0.00" json:"subtotal"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ValidTransitions defines allowed next states from any given state
var ValidTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusPending:    {OrderStatusConfirmed, OrderStatusCancelled},
	OrderStatusConfirmed:  {OrderStatusProcessing, OrderStatusCancelled},
	OrderStatusProcessing: {OrderStatusShipped, OrderStatusCancelled},
	OrderStatusShipped:    {OrderStatusDelivered},
	OrderStatusDelivered:  {}, // Final state
	OrderStatusCancelled:  {}, // Final state
}

// CanTransitionTo checks if transition from current to target state is legally permitted
func (o *Order) CanTransitionTo(next OrderStatus) bool {
	allowed, exists := ValidTransitions[o.Status]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if transition is not permitted
func (o *Order) ValidateTransition(next OrderStatus) error {
	if !o.CanTransitionTo(next) {
		return errors.New("invalid order state transition from " + string(o.Status) + " to " + string(next))
	}
	return nil
}
