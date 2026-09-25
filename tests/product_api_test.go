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
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/pkg/jwt"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// MockCategoryRepo implements repositories.CategoryRepository for fast in-memory testing
type MockCategoryRepo struct {
	categories map[uint]*models.Category
	nextID     uint
}

func NewMockCategoryRepo() *MockCategoryRepo {
	return &MockCategoryRepo{
		categories: make(map[uint]*models.Category),
		nextID:     1,
	}
}

func (m *MockCategoryRepo) GetByID(id uint) (*models.Category, error) {
	if cat, ok := m.categories[id]; ok {
		return cat, nil
	}
	return nil, nil
}

func (m *MockCategoryRepo) GetByName(name string) (*models.Category, error) {
	for _, cat := range m.categories {
		if strings.EqualFold(cat.Name, name) {
			return cat, nil
		}
	}
	return nil, nil
}

func (m *MockCategoryRepo) Create(category *models.Category) error {
	category.ID = m.nextID
	m.nextID++
	category.CreatedAt = time.Now().UTC()
	category.UpdatedAt = time.Now().UTC()
	m.categories[category.ID] = category
	return nil
}

func (m *MockCategoryRepo) Update(category *models.Category) error {
	category.UpdatedAt = time.Now().UTC()
	m.categories[category.ID] = category
	return nil
}

func (m *MockCategoryRepo) Delete(id uint) error {
	delete(m.categories, id)
	return nil
}

func (m *MockCategoryRepo) List() ([]models.Category, error) {
	var list []models.Category
	for _, c := range m.categories {
		list = append(list, *c)
	}
	return list, nil
}

// MockProductRepo implements repositories.ProductRepository
type MockProductRepo struct {
	products map[uint]*models.Product
	nextID   uint
}

func NewMockProductRepo() *MockProductRepo {
	return &MockProductRepo{
		products: make(map[uint]*models.Product),
		nextID:   1,
	}
}

func (m *MockProductRepo) GetByID(id uint) (*models.Product, error) {
	if p, ok := m.products[id]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *MockProductRepo) GetBySKU(sku string) (*models.Product, error) {
	for _, p := range m.products {
		if strings.EqualFold(p.SKU, sku) {
			return p, nil
		}
	}
	return nil, nil
}

func (m *MockProductRepo) Create(product *models.Product) error {
	product.ID = m.nextID
	m.nextID++
	product.CreatedAt = time.Now().UTC()
	product.UpdatedAt = time.Now().UTC()
	m.products[product.ID] = product
	return nil
}

func (m *MockProductRepo) Update(product *models.Product) error {
	product.UpdatedAt = time.Now().UTC()
	m.products[product.ID] = product
	return nil
}

func (m *MockProductRepo) Delete(id uint) error {
	if p, ok := m.products[id]; ok {
		p.Status = models.ProductStatusInactive
	}
	return nil
}

func (m *MockProductRepo) List(filter repositories.ProductFilter) ([]models.Product, int64, error) {
	var matched []models.Product
	for _, p := range m.products {
		// Filter by search
		if filter.Search != "" {
			term := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(p.Name), term) &&
				!strings.Contains(strings.ToLower(p.Description), term) &&
				!strings.Contains(strings.ToLower(p.SKU), term) {
				continue
			}
		}

		// Filter by CategoryID
		if filter.CategoryID > 0 && p.CategoryID != filter.CategoryID {
			continue
		}

		// Filter by MinPrice
		if filter.MinPrice != nil && p.Price < *filter.MinPrice {
			continue
		}

		// Filter by MaxPrice
		if filter.MaxPrice != nil && p.Price > *filter.MaxPrice {
			continue
		}

		// Filter by Status
		if filter.Status != "" && string(p.Status) != filter.Status {
			continue
		}

		matched = append(matched, *p)
	}

	total := int64(len(matched))
	return matched, total, nil
}

func (m *MockProductRepo) CountTotal() (int64, error) {
	return int64(len(m.products)), nil
}

func (m *MockProductRepo) CountLowStock(threshold int) (int64, error) {
	var count int64
	for _, p := range m.products {
		if p.AvailableStock() > 0 && p.AvailableStock() <= threshold {
			count++
		}
	}
	return count, nil
}

func (m *MockProductRepo) CountOutOfStock() (int64, error) {
	var count int64
	for _, p := range m.products {
		if p.AvailableStock() <= 0 {
			count++
		}
	}
	return count, nil
}

func (m *MockProductRepo) GetLowStockProducts(threshold int, limit int) ([]models.Product, error) {
	var items []models.Product
	for _, p := range m.products {
		if p.AvailableStock() <= threshold {
			items = append(items, *p)
		}
	}
	return items, nil
}

// MockInventoryRepo implements repositories.InventoryRepository
type MockInventoryRepo struct {
	transactions []models.InventoryTransaction
}

func (m *MockInventoryRepo) CreateTransaction(tx *gorm.DB, transaction *models.InventoryTransaction) error {
	m.transactions = append(m.transactions, *transaction)
	return nil
}
func (m *MockInventoryRepo) GetProductTransactions(productID uint, page, limit int) ([]models.InventoryTransaction, int64, error) {
	return nil, 0, nil
}
func (m *MockInventoryRepo) ListAllTransactions(page, limit int) ([]models.InventoryTransaction, int64, error) {
	return nil, 0, nil
}
func (m *MockInventoryRepo) Restock(productID uint, quantity int, notes string) (*models.Product, error) {
	return &models.Product{ID: productID, Stock: quantity, Name: "Restocked Product"}, nil
}
func (m *MockInventoryRepo) ReserveStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error {
	return nil
}
func (m *MockInventoryRepo) ReleaseStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error {
	return nil
}
func (m *MockInventoryRepo) CommitReservedStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error {
	return nil
}

func setupTestApp() (*fiber.App, services.ProductService, services.CategoryService, string) {
	catRepo := NewMockCategoryRepo()
	prodRepo := NewMockProductRepo()
	invRepo := &MockInventoryRepo{}

	catService := services.NewCategoryService(catRepo)
	prodService := services.NewProductService(prodRepo, catRepo, invRepo)

	testSecret := "test-jwt-secret-key-32-chars-long"
	adminToken, _ := jwt.GenerateToken(1, "admin@producthub.com", models.RoleAdmin, testSecret, 24)

	app := api.SetupRouter(api.RouterConfig{
		DB:                 nil,
		JWTSecret:          testSecret,
		JWTExpirationHours: 24,
		CategoryService:    catService,
		ProductService:     prodService,
	})

	return app, prodService, catService, adminToken
}

func TestCategoryAPI_CreateAndList(t *testing.T) {
	app, _, _, adminToken := setupTestApp()

	// 1. Create Category (Requires Admin Token)
	catPayload := map[string]string{
		"name":        "Electronics",
		"description": "Smartphones and computing hardware",
	}
	body, _ := json.Marshal(catPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/categories", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	var res response.Response
	respBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBytes, &res)

	if !res.Success {
		t.Errorf("Expected success true, got %v", res.Success)
	}

	// 2. List Categories (Public)
	reqList := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	respList, _ := app.Test(reqList, -1)

	if respList.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", respList.StatusCode)
	}
}

func TestProductAPI_CreateValidationAndListing(t *testing.T) {
	app, _, catService, adminToken := setupTestApp()

	// Seed Category
	cat, err := catService.CreateCategory("Computers", "Workstation hardware")
	if err != nil {
		t.Fatalf("Failed to seed category: %v", err)
	}

	// 1. Reject invalid product (negative price)
	invalidProd := map[string]interface{}{
		"sku":         "TEST-SKU-1",
		"name":        "Test Laptop",
		"category_id": cat.ID,
		"price":       -100.0,
		"stock":       10,
	}
	body, _ := json.Marshal(invalidProd)
	req := httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected status 422 for negative price, got %d", resp.StatusCode)
	}

	// 2. Create valid product
	validProd := map[string]interface{}{
		"sku":         "PROD-LAPTOP-X1",
		"name":        "ThinkPad X1 Carbon Gen 12",
		"description": "Ultra-lightweight premium business laptop",
		"category_id": cat.ID,
		"price":       1899.99,
		"stock":       25,
	}
	body, _ = json.Marshal(validProd)
	req = httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 for valid product, got %d", resp.StatusCode)
	}

	// 3. Query Product with filter, search, sort, and pagination (Public)
	queryURL := fmt.Sprintf("/api/products?page=1&limit=10&search=ThinkPad&min_price=1000&max_price=2000&sort=price&order=desc")
	reqGet := httptest.NewRequest(http.MethodGet, queryURL, nil)

	respGet, _ := app.Test(reqGet, -1)
	if respGet.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", respGet.StatusCode)
	}

	var listRes response.Response
	respBytes, _ := io.ReadAll(respGet.Body)
	_ = json.Unmarshal(respBytes, &listRes)

	if !listRes.Success {
		t.Errorf("Expected success true, got %v", listRes.Success)
	}
	if listRes.Pagination == nil || listRes.Pagination.Total != 1 {
		t.Errorf("Expected 1 total item in pagination, got %v", listRes.Pagination)
	}

	// 4. Reject SQL Injection attempt in sort query
	sqlInjectURL := "/api/products?sort=price;DROP+TABLE+products;--"
	reqInject := httptest.NewRequest(http.MethodGet, sqlInjectURL, nil)
	respInject, _ := app.Test(reqInject, -1)

	if respInject.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for malicious sort parameter, got %d", respInject.StatusCode)
	}

	// 5. Test Soft Deactivation (DELETE /api/products/1 requires Admin Token)
	reqDelete := httptest.NewRequest(http.MethodDelete, "/api/products/1", nil)
	reqDelete.Header.Set("Authorization", "Bearer "+adminToken)
	respDelete, _ := app.Test(reqDelete, -1)

	if respDelete.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for product deactivation, got %d", respDelete.StatusCode)
	}
}
