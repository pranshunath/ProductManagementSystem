package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"producthub/internal/api"
	"producthub/internal/models"
	"producthub/internal/services"
	"producthub/internal/workers"
	pkgjwt "producthub/pkg/jwt"
)

func TestAdminDashboard_StaticAssetServing(t *testing.T) {
	app := api.SetupRouter(api.RouterConfig{
		DB: nil,
	})

	// 1. Verify GET /admin.html serves admin console HTML
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin.html", nil)
	respAdmin, err := app.Test(reqAdmin, -1)
	if err != nil {
		t.Fatalf("Failed to request /admin.html: %v", err)
	}
	if respAdmin.StatusCode != http.StatusOK {
		t.Fatalf("Expected GET /admin.html to return 200 OK, got %d", respAdmin.StatusCode)
	}
	bodyAdmin, err := io.ReadAll(respAdmin.Body)
	if err != nil {
		t.Fatalf("Failed to read body from /admin.html: %v", err)
	}
	if !strings.Contains(string(bodyAdmin), "Enterprise Operations Console") {
		t.Errorf("Expected admin HTML to contain 'Enterprise Operations Console'")
	}

	// 2. Verify GET /js/admin.js serves admin JS logic
	reqAdminJS := httptest.NewRequest(http.MethodGet, "/js/admin.js", nil)
	respAdminJS, err := app.Test(reqAdminJS, -1)
	if err != nil {
		t.Fatalf("Failed to request /js/admin.js: %v", err)
	}
	if respAdminJS.StatusCode != http.StatusOK {
		t.Fatalf("Expected GET /js/admin.js to return 200 OK, got %d", respAdminJS.StatusCode)
	}
	bodyAdminJS, err := io.ReadAll(respAdminJS.Body)
	if err != nil {
		t.Fatalf("Failed to read body from /js/admin.js: %v", err)
	}
	if !strings.Contains(string(bodyAdminJS), "adminState") {
		t.Errorf("Expected admin JS to contain 'adminState'")
	}
}

func TestAdminDashboard_RBAC_EndpointAccessControl(t *testing.T) {
	jwtSecret := "test-admin-secret-key-123456"

	// Worker Pool setup
	pool := workers.NewWorkerPool(4, 100)
	pool.Start(context.Background())
	defer func() { _ = pool.Stop(1 * time.Second) }()

	// Mocks
	userRepo := NewMockUserRepo()
	userService := services.NewUserService(userRepo)

	orderRepo := NewMockOrderRepo()
	orderService := services.NewOrderService(orderRepo, nil, nil, userRepo)

	app := api.SetupRouter(api.RouterConfig{
		JWTSecret:    jwtSecret,
		UserService:  userService,
		OrderService: orderService,
		WorkerPool:   pool,
	})

	// Generate test tokens
	customerToken, err := pkgjwt.GenerateToken(10, "customer@example.com", models.RoleCustomer, jwtSecret, 1)
	if err != nil {
		t.Fatalf("Failed to generate customer token: %v", err)
	}
	adminToken, err := pkgjwt.GenerateToken(1, "admin@example.com", models.RoleAdmin, jwtSecret, 1)
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	// Test 1: Worker Pool stats endpoint
	// 1a: Unauthenticated -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/admin/workers/stats", nil)
	respUnauth, _ := app.Test(reqUnauth, -1)
	if respUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated worker stats, got %d", respUnauth.StatusCode)
	}

	// 1b: Customer -> 403 Forbidden
	reqCust := httptest.NewRequest(http.MethodGet, "/api/admin/workers/stats", nil)
	reqCust.Header.Set("Authorization", "Bearer "+customerToken)
	respCust, _ := app.Test(reqCust, -1)
	if respCust.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for customer accessing worker stats, got %d", respCust.StatusCode)
	}

	// 1c: Admin -> 200 OK with worker stats
	reqAdm := httptest.NewRequest(http.MethodGet, "/api/admin/workers/stats", nil)
	reqAdm.Header.Set("Authorization", "Bearer "+adminToken)
	respAdm, _ := app.Test(reqAdm, -1)
	if respAdm.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for admin accessing worker stats, got %d", respAdm.StatusCode)
	}

	var statsRes struct {
		Success bool `json:"success"`
		Data    struct {
			WorkerCount   int  `json:"worker_count"`
			QueueCapacity int  `json:"queue_capacity"`
			IsRunning     bool `json:"is_running"`
		} `json:"data"`
	}
	bodyStats, _ := io.ReadAll(respAdm.Body)
	_ = json.Unmarshal(bodyStats, &statsRes)
	if !statsRes.Success || !statsRes.Data.IsRunning || statsRes.Data.WorkerCount != 4 {
		t.Errorf("Expected worker pool running with 4 workers, got %+v", statsRes)
	}

	// Test 2: Admin Orders listing endpoint
	// 2a: Unauthenticated -> 401
	reqOrdUnauth := httptest.NewRequest(http.MethodGet, "/api/admin/orders", nil)
	respOrdUnauth, _ := app.Test(reqOrdUnauth, -1)
	if respOrdUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated admin orders, got %d", respOrdUnauth.StatusCode)
	}

	// 2b: Customer -> 403
	reqOrdCust := httptest.NewRequest(http.MethodGet, "/api/admin/orders", nil)
	reqOrdCust.Header.Set("Authorization", "Bearer "+customerToken)
	respOrdCust, _ := app.Test(reqOrdCust, -1)
	if respOrdCust.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for customer accessing admin orders, got %d", respOrdCust.StatusCode)
	}

	// 2c: Admin -> 200 OK
	reqOrdAdm := httptest.NewRequest(http.MethodGet, "/api/admin/orders", nil)
	reqOrdAdm.Header.Set("Authorization", "Bearer "+adminToken)
	respOrdAdm, _ := app.Test(reqOrdAdm, -1)
	if respOrdAdm.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for admin accessing admin orders, got %d", respOrdAdm.StatusCode)
	}
}
