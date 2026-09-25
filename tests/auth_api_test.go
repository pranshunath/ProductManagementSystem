package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"producthub/internal/api"
	"producthub/internal/models"
	"producthub/internal/services"
	"producthub/pkg/jwt"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// MockUserRepo implements repositories.UserRepository for unit testing
type MockUserRepo struct {
	users  map[uint]*models.User
	nextID uint
}

func NewMockUserRepo() *MockUserRepo {
	return &MockUserRepo{
		users:  make(map[uint]*models.User),
		nextID: 1,
	}
}

func (m *MockUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *MockUserRepo) GetByEmail(email string) (*models.User, error) {
	for _, u := range m.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return nil, nil
}

func (m *MockUserRepo) Create(user *models.User) error {
	user.ID = m.nextID
	m.nextID++
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = time.Now().UTC()
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepo) Update(user *models.User) error {
	user.UpdatedAt = time.Now().UTC()
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepo) List(page, limit int) ([]models.User, int64, error) {
	var list []models.User
	for _, u := range m.users {
		list = append(list, *u)
	}
	return list, int64(len(list)), nil
}

func (m *MockUserRepo) CountCustomers() (int64, error) {
	var count int64
	for _, u := range m.users {
		if u.Role == models.RoleCustomer {
			count++
		}
	}
	return count, nil
}

func setupAuthTestApp() (*fiber.App, services.UserService, services.ProductService, services.CategoryService, string) {
	userRepo := NewMockUserRepo()
	catRepo := NewMockCategoryRepo()
	prodRepo := NewMockProductRepo()
	invRepo := &MockInventoryRepo{}

	testSecret := "super-secure-test-jwt-secret-key-32-chars"
	userService := services.NewUserService(userRepo)
	catService := services.NewCategoryService(catRepo)
	prodService := services.NewProductService(prodRepo, catRepo, invRepo)

	app := api.SetupRouter(api.RouterConfig{
		DB:                 nil,
		JWTSecret:          testSecret,
		JWTExpirationHours: 24,
		UserService:        userService,
		CategoryService:    catService,
		ProductService:     prodService,
	})

	return app, userService, prodService, catService, testSecret
}

func TestAuth_RegisterAndLogin(t *testing.T) {
	app, _, _, _, _ := setupAuthTestApp()

	// 1. Register customer
	regPayload := map[string]string{
		"name":     "Alice Wonderland",
		"email":    "alice@example.com",
		"password": "Password123!",
	}
	body, _ := json.Marshal(regPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	var regRes response.Response
	respBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBytes, &regRes)

	dataMap, ok := regRes.Data.(map[string]interface{})
	if !ok || dataMap["token"] == "" {
		t.Fatalf("Expected token in register response: %v", regRes.Data)
	}

	// 2. Login with correct credentials
	loginPayload := map[string]string{
		"email":    "alice@example.com",
		"password": "Password123!",
	}
	body, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// 3. Login with incorrect password
	badLogin := map[string]string{
		"email":    "alice@example.com",
		"password": "WrongPassword",
	}
	body, _ = json.Marshal(badLogin)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for bad password, got %d", resp.StatusCode)
	}
}

func TestAuth_ProtectedEndpoint_Me(t *testing.T) {
	app, userService, _, _, testSecret := setupAuthTestApp()

	// Create user
	user, err := userService.Register("Bob Builder", "bob@example.com", "SecretPass123", models.RoleCustomer)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Generate Token
	token, _ := jwt.GenerateToken(user.ID, user.Email, user.Role, testSecret, 24)

	// 1. Unauthenticated request to /api/auth/me
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	respNoAuth, _ := app.Test(reqNoAuth, -1)
	if respNoAuth.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for missing token, got %d", respNoAuth.StatusCode)
	}

	// 2. Authenticated request with Bearer token
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqAuth.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	respAuth, _ := app.Test(reqAuth, -1)
	if respAuth.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for valid token, got %d", respAuth.StatusCode)
	}
}

func TestAuth_RoleBasedAccessControl(t *testing.T) {
	app, _, _, catService, testSecret := setupAuthTestApp()

	cat, _ := catService.CreateCategory("Gadgets", "Smart accessories")

	customerToken, _ := jwt.GenerateToken(10, "customer@test.com", models.RoleCustomer, testSecret, 24)
	adminToken, _ := jwt.GenerateToken(1, "admin@test.com", models.RoleAdmin, testSecret, 24)

	prodPayload := map[string]interface{}{
		"sku":         "RBAC-TEST-1",
		"name":        "RBAC Drone",
		"category_id": cat.ID,
		"price":       799.00,
		"stock":       5,
	}
	body, _ := json.Marshal(prodPayload)

	// 1. Customer token should be REJECTED with 403 Forbidden
	reqCust := httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewReader(body))
	reqCust.Header.Set("Content-Type", "application/json")
	reqCust.Header.Set("Authorization", "Bearer "+customerToken)

	respCust, _ := app.Test(reqCust, -1)
	if respCust.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for customer role attempting admin operation, got %d", respCust.StatusCode)
	}

	// 2. Admin token should be ACCEPTED with 201 Created
	reqAdmin := httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewReader(body))
	reqAdmin.Header.Set("Content-Type", "application/json")
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)

	respAdmin, _ := app.Test(reqAdmin, -1)
	if respAdmin.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 Created for admin token, got %d", respAdmin.StatusCode)
	}
}
