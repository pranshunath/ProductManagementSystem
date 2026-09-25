package workers

import (
	"context"
	"log"
	"time"
)

// RegisterDefaultHandlers attaches the built-in domain event handlers to the pool
func RegisterDefaultHandlers(pool *WorkerPool) {
	pool.RegisterHandler(EventOrderCreated, HandleOrderCreated)
	pool.RegisterHandler(EventOrderCancelled, HandleOrderCancelled)
	pool.RegisterHandler(EventOrderStatusChanged, HandleOrderStatusChanged)
	pool.RegisterHandler(EventLowStockAlert, HandleLowStockAlert)
}

// HandleOrderCreated simulates background asynchronous order confirmation and invoice generation
func HandleOrderCreated(ctx context.Context, e Event) error {
	orderID := e.Payload["order_id"]
	userID := e.Payload["user_id"]
	total := e.Payload["total_amount"]
	itemsCount := e.Payload["items_count"]

	log.Printf("[ASYNC WORKER] -> Order Confirmation: Generated invoice for Order #%v (User: %v, Total: $%.2f, Items: %v)",
		orderID, userID, total, itemsCount)

	// Simulate external service call (e.g. SMTP email sender, SMS gateway)
	select {
	case <-time.After(20 * time.Millisecond):
		log.Printf("[ASYNC WORKER] -> Email receipt dispatched to customer for Order #%v", orderID)
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// HandleOrderCancelled handles background notifications when an order is cancelled
func HandleOrderCancelled(ctx context.Context, e Event) error {
	orderID := e.Payload["order_id"]
	userID := e.Payload["user_id"]

	log.Printf("[ASYNC WORKER] -> Cancellation: Notification and refund processing dispatched for Order #%v (User: %v)",
		orderID, userID)

	return nil
}

// HandleOrderStatusChanged notifies the customer on shipment or delivery status updates
func HandleOrderStatusChanged(ctx context.Context, e Event) error {
	orderID := e.Payload["order_id"]
	status := e.Payload["status"]

	log.Printf("[ASYNC WORKER] -> Order Tracking: Status updated to [%s] for Order #%v", status, orderID)

	return nil
}

// HandleLowStockAlert fires an urgent alert when product available stock drops below threshold
func HandleLowStockAlert(ctx context.Context, e Event) error {
	prodID := e.Payload["product_id"]
	sku := e.Payload["sku"]
	name := e.Payload["name"]
	remaining := e.Payload["remaining_stock"]

	log.Printf("[ASYNC ALERT - PROCUREMENT] Product '%s' (SKU: %s, ID: %v) has LOW STOCK (%v units left). Procurement notification sent!",
		name, sku, prodID, remaining)

	return nil
}
