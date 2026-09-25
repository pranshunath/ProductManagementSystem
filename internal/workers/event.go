package workers

import (
	"time"

	"github.com/google/uuid"
)

// EventType identifies the category of domain event
type EventType string

const (
	EventOrderCreated       EventType = "ORDER_CREATED"
	EventOrderCancelled     EventType = "ORDER_CANCELLED"
	EventOrderStatusChanged EventType = "ORDER_STATUS_CHANGED"
	EventLowStockAlert      EventType = "LOW_STOCK_ALERT"
)

// Event represents a unit of asynchronous work dispatched through the queue
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
	Attempts  int                    `json:"attempts"`
}

// NewEvent creates a new Event instance with generated UUID and current timestamp
func NewEvent(eventType EventType, payload map[string]interface{}) Event {
	return Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
		Attempts:  0,
	}
}
