package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"producthub/internal/api"
	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/pkg/jwt"
	"producthub/pkg/response"
)

// ThreadSafeMockInventoryRepo simulates atomic database row-level locking
type ThreadSafeMockInventoryRepo struct {
	mu           sync.Mutex
	stock        int
	reserved     int
	transactions []models.InventoryTransaction
}

func NewThreadSafeMockInventoryRepo(initialStock int) *ThreadSafeMockInventoryRepo {
	return &ThreadSafeMockInventoryRepo{
		stock:    initialStock,
		reserved: 0,
	}
}

func (m *ThreadSafeMockInventoryRepo) ReserveStockAtomic(productID uint, quantity int, refID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Atomic check equivalent to MySQL InnoDB: WHERE id = ? AND (stock - reserved_stock) >= quantity
	available := m.stock - m.reserved
	if available < quantity {
		return repositories.ErrInsufficientStock
	}

	m.reserved += quantity
	m.transactions = append(m.transactions, models.InventoryTransaction{
		ProductID:   productID,
		Type:        models.TxTypeReserved,
		Quantity:    quantity,
		ReferenceID: refID,
	})
	return nil
}

func (m *ThreadSafeMockInventoryRepo) ReleaseStockAtomic(productID uint, quantity int, refID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.reserved < quantity {
		quantity = m.reserved
	}
	m.reserved -= quantity
	return nil
}

func (m *ThreadSafeMockInventoryRepo) AvailableStock() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stock - m.reserved
}

func (m *ThreadSafeMockInventoryRepo) ReservedStock() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reserved
}

func TestStockConcurrency_SingleItemRaceCondition(t *testing.T) {
	// Scenario: Stock = 1
	// 50 concurrent buyers simultaneously race to purchase/reserve the final 1 item.
	// System must allow EXACTLY 1 winner and 49 rejections.
	initialStock := 1
	concurrencyLimit := 50

	mockRepo := NewThreadSafeMockInventoryRepo(initialStock)

	var successCount int64
	var failureCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		go func(workerID int) {
			defer wg.Done()
			<-startBarrier // Release all goroutines simultaneously

			refID := fmt.Sprintf("ORDER-RACE-%d", workerID)
			err := mockRepo.ReserveStockAtomic(1, 1, refID)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failureCount, 1)
			}
		}(i)
	}

	// Trigger simultaneous execution
	close(startBarrier)
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("Race condition failure! Expected exactly 1 success, got %d", successCount)
	}

	if failureCount != int64(concurrencyLimit-1) {
		t.Errorf("Expected %d failures, got %d", concurrencyLimit-1, failureCount)
	}

	if mockRepo.AvailableStock() != 0 {
		t.Errorf("Expected 0 available stock, got %d", mockRepo.AvailableStock())
	}

	if mockRepo.ReservedStock() != 1 {
		t.Errorf("Expected 1 reserved stock, got %d", mockRepo.ReservedStock())
	}
}

func TestStockConcurrency_MultipleStockRaceCondition(t *testing.T) {
	// Scenario: Stock = 5
	// 100 concurrent buyers simultaneously race for 5 items.
	// Exactly 5 must succeed; 95 must be safely rejected.
	initialStock := 5
	concurrencyLimit := 100

	mockRepo := NewThreadSafeMockInventoryRepo(initialStock)

	var successCount int64
	var failureCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		go func(workerID int) {
			defer wg.Done()
			<-startBarrier

			refID := fmt.Sprintf("MULTI-ORDER-%d", workerID)
			err := mockRepo.ReserveStockAtomic(1, 1, refID)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failureCount, 1)
			}
		}(i)
	}

	close(startBarrier)
	wg.Wait()

	if successCount != int64(initialStock) {
		t.Fatalf("Expected exactly %d successes, got %d", initialStock, successCount)
	}

	if failureCount != int64(concurrencyLimit-initialStock) {
		t.Errorf("Expected %d failures, got %d", concurrencyLimit-initialStock, failureCount)
	}

	if mockRepo.AvailableStock() != 0 {
		t.Errorf("Expected 0 available stock remaining, got %d", mockRepo.AvailableStock())
	}

	if mockRepo.ReservedStock() != initialStock {
		t.Errorf("Expected %d reserved stock, got %d", initialStock, mockRepo.ReservedStock())
	}
}

func TestInventoryAPI_RestockAndSummary(t *testing.T) {
	catRepo := NewMockCategoryRepo()
	prodRepo := NewMockProductRepo()
	invRepo := &MockInventoryRepo{}

	catService := services.NewCategoryService(catRepo)
	prodService := services.NewProductService(prodRepo, catRepo, invRepo)
	invService := services.NewInventoryService(invRepo, prodRepo)

	testSecret := "test-secret-inventory-jwt-key"
	adminToken, _ := jwt.GenerateToken(1, "admin@producthub.com", models.RoleAdmin, testSecret, 24)
	customerToken, _ := jwt.GenerateToken(2, "user@producthub.com", models.RoleCustomer, testSecret, 24)

	app := api.SetupRouter(api.RouterConfig{
		DB:                 nil,
		JWTSecret:          testSecret,
		JWTExpirationHours: 24,
		CategoryService:    catService,
		ProductService:     prodService,
		InventoryService:   invService,
	})

	// Seed category and low-stock product
	cat, _ := catService.CreateCategory("Audio", "Headphones")
	prod, _ := prodService.CreateProduct("AUDIO-01", "Earbuds", "Compact", "", cat.ID, 49.99, 2)

	// 1. Customer attempting restock -> Expect 403 Forbidden
	restockBody, _ := json.Marshal(map[string]interface{}{
		"product_id": prod.ID,
		"quantity":   50,
		"notes":      "Batch shipment 104",
	})
	reqCust := httptest.NewRequest(http.MethodPost, "/api/inventory/restock", bytes.NewReader(restockBody))
	reqCust.Header.Set("Content-Type", "application/json")
	reqCust.Header.Set("Authorization", "Bearer "+customerToken)

	respCust, _ := app.Test(reqCust, -1)
	if respCust.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for customer trying to restock, got %d", respCust.StatusCode)
	}

	// 2. Public stock check endpoint
	reqStock := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/inventory/products/%d/stock", prod.ID), nil)
	respStock, _ := app.Test(reqStock, -1)
	if respStock.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for stock check, got %d", respStock.StatusCode)
	}

	var stockRes response.Response
	respBytes, _ := io.ReadAll(respStock.Body)
	_ = json.Unmarshal(respBytes, &stockRes)
	dataMap := stockRes.Data.(map[string]interface{})
	if dataMap["available_stock"].(float64) != 2 {
		t.Errorf("Expected 2 available stock, got %v", dataMap["available_stock"])
	}

	// 3. Admin Inventory Summary
	reqSummary := httptest.NewRequest(http.MethodGet, "/api/inventory/summary", nil)
	reqSummary.Header.Set("Authorization", "Bearer "+adminToken)
	respSummary, _ := app.Test(reqSummary, -1)
	if respSummary.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for admin inventory summary, got %d", respSummary.StatusCode)
	}
}
