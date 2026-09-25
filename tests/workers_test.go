package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"producthub/internal/api"
	"producthub/internal/models"
	"producthub/internal/services"
	"producthub/internal/workers"
	"producthub/pkg/jwt"
)

func TestWorkerPool_ConcurrentProcessing(t *testing.T) {
	workerCount := 4
	queueCap := 100
	pool := workers.NewWorkerPool(workerCount, queueCap)

	var processedCount int64
	var mu sync.Mutex
	receivedEvents := make(map[string]bool)

	pool.RegisterHandler(workers.EventOrderCreated, func(ctx context.Context, e workers.Event) error {
		atomic.AddInt64(&processedCount, 1)
		mu.Lock()
		receivedEvents[e.ID] = true
		mu.Unlock()
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Concurrently dispatch 50 events from 5 goroutines
	numJobs := 50
	var wg sync.WaitGroup
	wg.Add(5)

	for g := 0; g < 5; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				evt := workers.NewEvent(workers.EventOrderCreated, map[string]interface{}{
					"job_index": i,
					"sender":    gID,
				})
				ok := pool.Dispatch(evt)
				if !ok {
					t.Errorf("Dispatch failed for event %s", evt.ID)
				}
			}
		}(g)
	}

	wg.Wait()

	// Wait briefly for all workers to process jobs
	time.Sleep(100 * time.Millisecond)

	err := pool.Stop(2 * time.Second)
	if err != nil {
		t.Fatalf("Worker pool stop returned error: %v", err)
	}

	if atomic.LoadInt64(&processedCount) != int64(numJobs) {
		t.Errorf("Expected %d processed events, got %d", numJobs, processedCount)
	}

	mu.Lock()
	if len(receivedEvents) != numJobs {
		t.Errorf("Expected %d unique events, got %d", numJobs, len(receivedEvents))
	}
	mu.Unlock()

	stats := pool.Stats()
	if stats.ProcessedCount != int64(numJobs) {
		t.Errorf("Expected stats.ProcessedCount %d, got %d", numJobs, stats.ProcessedCount)
	}
	if stats.IsRunning {
		t.Errorf("Expected pool.IsRunning to be false after Stop()")
	}
}

func TestWorkerPool_GracefulShutdownAndDrain(t *testing.T) {
	pool := workers.NewWorkerPool(2, 50)

	var executedCount int64
	pool.RegisterHandler(workers.EventOrderCancelled, func(ctx context.Context, e workers.Event) error {
		// Simulate non-trivial work
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt64(&executedCount, 1)
		return nil
	})

	pool.Start(context.Background())

	// Dispatch 15 items into queue
	eventCount := 15
	for i := 0; i < eventCount; i++ {
		pool.Dispatch(workers.NewEvent(workers.EventOrderCancelled, map[string]interface{}{
			"order_id": i + 1,
		}))
	}

	// Immediately call Stop()
	// Channel will be closed and workers MUST drain all 15 remaining items before Stop() returns
	err := pool.Stop(3 * time.Second)
	if err != nil {
		t.Fatalf("Stop() failed with error: %v", err)
	}

	if atomic.LoadInt64(&executedCount) != int64(eventCount) {
		t.Errorf("Queue did not drain completely! Expected %d executed events, got %d", eventCount, executedCount)
	}
}

func TestWorkerPool_OrderEventFlow(t *testing.T) {
	pool := workers.NewWorkerPool(2, 50)

	var orderCreatedReceived atomic.Bool
	var lowStockReceived atomic.Bool
	var capturedOrderID uint
	var capturedLowStockProdID uint

	pool.RegisterHandler(workers.EventOrderCreated, func(ctx context.Context, e workers.Event) error {
		orderCreatedReceived.Store(true)
		if oid, ok := e.Payload["order_id"].(uint); ok {
			capturedOrderID = oid
		}
		return nil
	})

	pool.RegisterHandler(workers.EventLowStockAlert, func(ctx context.Context, e workers.Event) error {
		lowStockReceived.Store(true)
		if pid, ok := e.Payload["product_id"].(uint); ok {
			capturedLowStockProdID = pid
		}
		return nil
	})

	pool.Start(context.Background())
	defer func() { _ = pool.Stop(1 * time.Second) }()

	userRepo := NewMockUserRepo()
	prodRepo := NewMockProductRepo()
	orderRepo := NewMockOrderRepo()
	cartRepo := NewMockCartRepo()

	userService := services.NewUserService(userRepo)
	prodService := services.NewProductService(prodRepo, nil, nil)
	orderService := &InMemOrderService{orderRepo: orderRepo, prodRepo: prodRepo}
	cartService := services.NewCartService(cartRepo, prodRepo)

	customer := &models.User{
		ID:    3,
		Name:  "Worker Customer",
		Email: "worker_cust@example.com",
		Role:  models.RoleCustomer,
	}
	_ = userRepo.Create(customer)
	customerToken, _ := jwt.GenerateToken(customer.ID, customer.Email, customer.Role, "test-secret-key-123", 24)

	// Seed product with initial stock = 7; purchasing 3 leaves available stock = 4 (triggering LOW_STOCK_ALERT <= 5)
	prod := &models.Product{
		ID:     20,
		SKU:    "LOW-STOCK-01",
		Name:   "Low Stock Item",
		Price:  25.0,
		Stock:  7,
		Status: models.ProductStatusActive,
	}
	_ = prodRepo.Create(prod)

	app := api.SetupRouter(api.RouterConfig{
		JWTSecret:      "test-secret-key-123",
		UserService:    userService,
		ProductService: prodService,
		OrderService:   orderService,
		CartService:    cartService,
		WorkerPool:     pool,
	})

	checkoutPayload, _ := json.Marshal(map[string]interface{}{
		"shipping_address": "456 Async Avenue, Silicon City",
		"payment_method":   "UPI",
		"items": []map[string]interface{}{
			{
				"product_id": prod.ID,
				"quantity":   3,
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(checkoutPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+customerToken)

	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("Checkout failed: %v, status: %d", err, resp.StatusCode)
	}

	// Give worker goroutines a moment to process the dispatched events
	time.Sleep(80 * time.Millisecond)

	if !orderCreatedReceived.Load() {
		t.Errorf("Expected EventOrderCreated to be processed by background worker")
	}

	if capturedOrderID == 0 {
		t.Errorf("Expected non-zero order ID in EventOrderCreated payload")
	}

	if !lowStockReceived.Load() {
		t.Errorf("Expected EventLowStockAlert to be dispatched when available stock falls <= 5")
	}

	if capturedLowStockProdID != prod.ID {
		t.Errorf("Expected low stock product ID %d, got %d", prod.ID, capturedLowStockProdID)
	}

	// Verify remaining stock is 4 (7 - 3 = 4)
	pAfter, _ := prodRepo.GetByID(prod.ID)
	if pAfter.Stock != 4 {
		t.Errorf("Expected stock to be 4, got %d", pAfter.Stock)
	}
}

func TestWorkerPool_AdminStatsEndpoint(t *testing.T) {
	pool := workers.NewWorkerPool(4, 100)
	workers.RegisterDefaultHandlers(pool)
	pool.Start(context.Background())
	defer func() { _ = pool.Stop(1 * time.Second) }()

	userRepo := NewMockUserRepo()
	userService := services.NewUserService(userRepo)

	admin := &models.User{
		ID:    1,
		Name:  "Admin Tester",
		Email: "admin_worker@producthub.local",
		Role:  models.RoleAdmin,
	}
	_ = userRepo.Create(admin)
	adminToken, _ := jwt.GenerateToken(admin.ID, admin.Email, admin.Role, "test-secret-key-123", 24)

	app := api.SetupRouter(api.RouterConfig{
		JWTSecret:   "test-secret-key-123",
		UserService: userService,
		WorkerPool:  pool,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/workers/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to fetch worker stats: %v, status: %d", err, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var respMap map[string]interface{}
	_ = json.Unmarshal(body, &respMap)

	data := respMap["data"].(map[string]interface{})
	if data["worker_count"].(float64) != 4 {
		t.Errorf("Expected worker_count: 4, got %v", data["worker_count"])
	}
	if data["queue_capacity"].(float64) != 100 {
		t.Errorf("Expected queue_capacity: 100, got %v", data["queue_capacity"])
	}
	if !data["is_running"].(bool) {
		t.Errorf("Expected is_running: true")
	}
}
