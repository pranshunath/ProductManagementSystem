package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"producthub/internal/api"
	"producthub/internal/cache"
	"producthub/internal/middleware"
	"producthub/internal/models"
	"producthub/internal/services"
	"producthub/pkg/jwt"

	"github.com/gofiber/fiber/v2"
)

func TestRedis_CacheAside_ProductCatalog(t *testing.T) {
	// Initialize in-memory cache service (implements cache.CacheService)
	cacheService := cache.NewMemoryCache()

	// Initialize mock repositories and services
	userRepo := NewMockUserRepo()
	catRepo := NewMockCategoryRepo()
	prodRepo := NewMockProductRepo()
	invRepo := &MockInventoryRepo{}

	userService := services.NewUserService(userRepo)
	catService := services.NewCategoryService(catRepo)
	prodService := services.NewProductService(prodRepo, catRepo, invRepo)
	invService := services.NewInventoryService(invRepo, prodRepo)

	// Seed admin user, category and product
	adminUser := &models.User{
		ID:    1,
		Name:  "Admin User",
		Email: "admin@producthub.local",
		Role:  models.RoleAdmin,
	}
	_ = userRepo.Create(adminUser)
	adminToken, _ := jwt.GenerateToken(adminUser.ID, adminUser.Email, adminUser.Role, "test-secret-key-123", 24)

	cat := &models.Category{ID: 1, Name: "Electronics", Description: "Electronic items"}
	_ = catRepo.Create(cat)

	prod := &models.Product{
		ID:         1,
		SKU:        "PROD-CACHE-01",
		Name:       "Cacheable Laptop",
		CategoryID: cat.ID,
		Price:      1299.99,
		Stock:      20,
		Status:     models.ProductStatusActive,
	}
	_ = prodRepo.Create(prod)

	app := api.SetupRouter(api.RouterConfig{
		JWTSecret:        "test-secret-key-123",
		UserService:      userService,
		CategoryService:  catService,
		ProductService:   prodService,
		InventoryService: invService,
		CacheService:     cacheService,
	})

	// 1. First GET request -> Cache MISS (fetches from service, stores in Redis/Memory)
	req1 := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	resp1, err := app.Test(req1, -1)
	if err != nil {
		t.Fatalf("Failed to execute request 1: %v", err)
	}
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp1.StatusCode)
	}

	xCache1 := resp1.Header.Get("X-Cache")
	if xCache1 != "MISS" {
		t.Errorf("Expected X-Cache: MISS on first request, got: '%s'", xCache1)
	}

	// 2. Second GET request -> Cache HIT (served straight from cache)
	req2 := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("Failed to execute request 2: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp2.StatusCode)
	}

	xCache2 := resp2.Header.Get("X-Cache")
	if xCache2 != "HIT" {
		t.Errorf("Expected X-Cache: HIT on second request, got: '%s'", xCache2)
	}

	// 3. Update Product -> Invalidate Cache
	updatePayload, _ := json.Marshal(map[string]interface{}{
		"name":  "Cacheable Laptop Ultra",
		"price": 1499.99,
	})
	reqUpdate := httptest.NewRequest(http.MethodPut, "/api/products/1", bytes.NewReader(updatePayload))
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate.Header.Set("Authorization", "Bearer "+adminToken)

	respUpdate, err := app.Test(reqUpdate, -1)
	if err != nil || respUpdate.StatusCode != http.StatusOK {
		t.Fatalf("Product update failed: %v, status: %d", err, respUpdate.StatusCode)
	}

	// 4. Third GET request -> Cache MISS again (because cache was invalidated by update!)
	req3 := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("Failed to execute request 3: %v", err)
	}
	xCache3 := resp3.Header.Get("X-Cache")
	if xCache3 != "MISS" {
		t.Errorf("Expected X-Cache: MISS after product update invalidation, got: '%s'", xCache3)
	}

	// 5. Fourth GET request -> Cache HIT with updated name and price
	req4 := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	resp4, _ := app.Test(req4, -1)
	if resp4.Header.Get("X-Cache") != "HIT" {
		t.Errorf("Expected X-Cache: HIT on request 4, got: '%s'", resp4.Header.Get("X-Cache"))
	}

	bodyBytes, _ := io.ReadAll(resp4.Body)
	var getResp map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &getResp)
	data := getResp["data"].(map[string]interface{})
	if data["name"] != "Cacheable Laptop Ultra" {
		t.Errorf("Expected updated product name in cached response, got: %v", data["name"])
	}

	// 6. Restock Product -> Invalidate Cache
	restockPayload, _ := json.Marshal(map[string]interface{}{
		"product_id": 1,
		"quantity":   50,
		"notes":      "Restock batch #10",
	})
	reqRestock := httptest.NewRequest(http.MethodPost, "/api/inventory/restock", bytes.NewReader(restockPayload))
	reqRestock.Header.Set("Content-Type", "application/json")
	reqRestock.Header.Set("Authorization", "Bearer "+adminToken)

	respRestock, err := app.Test(reqRestock, -1)
	if err != nil || respRestock.StatusCode != http.StatusOK {
		t.Fatalf("Restock failed: %v, status: %d", err, respRestock.StatusCode)
	}

	// 7. Fifth GET request -> Cache MISS after restock invalidation
	req5 := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	resp5, _ := app.Test(req5, -1)
	if resp5.Header.Get("X-Cache") != "MISS" {
		t.Errorf("Expected X-Cache: MISS after restock invalidation, got: '%s'", resp5.Header.Get("X-Cache"))
	}
}

func TestRedis_RateLimiter_Middleware(t *testing.T) {
	cacheService := cache.NewMemoryCache()

	app := fiber.New()
	app.Use(middleware.RateLimiter(cacheService, middleware.RateLimitConfig{
		Scope:  "test_rate_limit",
		Max:    3,
		Window: 1 * time.Minute,
	}))

	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})

	// Request 1: Allowed (Remaining = 2)
	req1 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp1, _ := app.Test(req1, -1)
	if resp1.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK on request 1, got %d", resp1.StatusCode)
	}
	if resp1.Header.Get("X-RateLimit-Limit") != "3" {
		t.Errorf("Expected X-RateLimit-Limit: 3, got %s", resp1.Header.Get("X-RateLimit-Limit"))
	}
	if resp1.Header.Get("X-RateLimit-Remaining") != "2" {
		t.Errorf("Expected X-RateLimit-Remaining: 2, got %s", resp1.Header.Get("X-RateLimit-Remaining"))
	}

	// Request 2: Allowed (Remaining = 1)
	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK on request 2, got %d", resp2.StatusCode)
	}
	if resp2.Header.Get("X-RateLimit-Remaining") != "1" {
		t.Errorf("Expected X-RateLimit-Remaining: 1, got %s", resp2.Header.Get("X-RateLimit-Remaining"))
	}

	// Request 3: Allowed (Remaining = 0)
	req3 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp3, _ := app.Test(req3, -1)
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK on request 3, got %d", resp3.StatusCode)
	}
	if resp3.Header.Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining: 0, got %s", resp3.Header.Get("X-RateLimit-Remaining"))
	}

	// Request 4: Blocked with HTTP 429 Too Many Requests
	req4 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp4, _ := app.Test(req4, -1)
	if resp4.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("Expected 429 Too Many Requests on request 4, got %d", resp4.StatusCode)
	}
	if resp4.Header.Get("Retry-After") == "" {
		t.Errorf("Expected Retry-After header on 429 response")
	}

	body4, _ := io.ReadAll(resp4.Body)
	var errResp map[string]interface{}
	_ = json.Unmarshal(body4, &errResp)
	errMap, ok := errResp["error"].(map[string]interface{})
	if !ok || errMap["code"] != "RATE_LIMIT_EXCEEDED" {
		t.Errorf("Expected error code RATE_LIMIT_EXCEEDED, got %v", errResp["error"])
	}
}

func TestRedis_Idempotency_OrderCheckout(t *testing.T) {
	cacheService := cache.NewMemoryCache()

	userRepo := NewMockUserRepo()
	prodRepo := NewMockProductRepo()
	orderRepo := NewMockOrderRepo()
	cartRepo := NewMockCartRepo()

	userService := services.NewUserService(userRepo)
	prodService := services.NewProductService(prodRepo, nil, nil)
	orderService := &InMemOrderService{orderRepo: orderRepo, prodRepo: prodRepo}
	cartService := services.NewCartService(cartRepo, prodRepo)

	// Seed customer
	customer := &models.User{
		ID:    5,
		Name:  "John Buyer",
		Email: "john@example.com",
		Role:  models.RoleCustomer,
	}
	_ = userRepo.Create(customer)
	customerToken, _ := jwt.GenerateToken(customer.ID, customer.Email, customer.Role, "test-secret-key-123", 24)

	// Seed product with 10 units
	prod := &models.Product{
		ID:     10,
		SKU:    "IDEM-PROD-01",
		Name:   "Idempotency Test Item",
		Price:  100.0,
		Stock:  10,
		Status: models.ProductStatusActive,
	}
	_ = prodRepo.Create(prod)

	app := api.SetupRouter(api.RouterConfig{
		JWTSecret:      "test-secret-key-123",
		UserService:    userService,
		ProductService: prodService,
		OrderService:   orderService,
		CartService:    cartService,
		CacheService:   cacheService,
	})

	idempotencyKey := "checkout-idemp-key-uuid-12345"
	checkoutPayload, _ := json.Marshal(map[string]interface{}{
		"shipping_address": "123 Test Boulevard, City",
		"payment_method":   "CREDIT_CARD",
		"items": []map[string]interface{}{
			{
				"product_id": prod.ID,
				"quantity":   2,
			},
		},
	})

	// 1. Initial checkout request with Idempotency-Key
	req1 := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(checkoutPayload))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+customerToken)
	req1.Header.Set("Idempotency-Key", idempotencyKey)

	resp1, err := app.Test(req1, -1)
	if err != nil {
		t.Fatalf("Failed to execute initial checkout: %v", err)
	}
	if resp1.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp1.Body)
		t.Fatalf("Expected 201 Created on initial checkout, got %d. Body: %s", resp1.StatusCode, string(body))
	}

	body1, _ := io.ReadAll(resp1.Body)
	var respObj1 map[string]interface{}
	_ = json.Unmarshal(body1, &respObj1)
	data1 := respObj1["data"].(map[string]interface{})
	orderID1 := data1["id"]

	// Verify stock was deducted from 10 to 8
	pAfterReq1, _ := prodRepo.GetByID(prod.ID)
	if pAfterReq1.Stock != 8 {
		t.Errorf("Expected stock to be 8 after initial order, got %d", pAfterReq1.Stock)
	}

	// 2. Duplicate retry request with the EXACT SAME Idempotency-Key
	req2 := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(checkoutPayload))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+customerToken)
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("Failed to execute duplicate checkout: %v", err)
	}

	// Should be served with X-Idempotency: HIT
	if resp2.Header.Get("X-Idempotency") != "HIT" {
		t.Errorf("Expected X-Idempotency: HIT on duplicate request, got: '%s'", resp2.Header.Get("X-Idempotency"))
	}

	body2, _ := io.ReadAll(resp2.Body)
	var respObj2 map[string]interface{}
	_ = json.Unmarshal(body2, &respObj2)
	data2 := respObj2["data"].(map[string]interface{})
	orderID2 := data2["id"]

	// Verify returned order is the exact same order
	if orderID1 != orderID2 {
		t.Errorf("Expected replay of order ID %v, got %v", orderID1, orderID2)
	}

	// CRITICAL: Stock MUST STILL BE 8 (NOT double-deducted to 6!)
	pAfterReq2, _ := prodRepo.GetByID(prod.ID)
	if pAfterReq2.Stock != 8 {
		t.Errorf("CRITICAL RACE HAZARD: Stock was double deducted to %d on idempotency replay!", pAfterReq2.Stock)
	}

	// 3. Test concurrent lock in-progress conflict (409 Conflict)
	conflictKey := "concurrent-test-key-555"
	_ = cacheService.Set(context.Background(), "idempotency:order:"+conflictKey, "IN_PROGRESS", 2*time.Minute)

	reqConflict := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(checkoutPayload))
	reqConflict.Header.Set("Content-Type", "application/json")
	reqConflict.Header.Set("Authorization", "Bearer "+customerToken)
	reqConflict.Header.Set("Idempotency-Key", conflictKey)

	respConflict, _ := app.Test(reqConflict, -1)
	if respConflict.StatusCode != http.StatusConflict {
		t.Errorf("Expected 409 Conflict for in-progress idempotency key, got %d", respConflict.StatusCode)
	}
}
