package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/pkg/response"
)

var (
	ErrSKUAlreadyExists = errors.New("SKU is already in use")
	ErrInvalidPrice     = errors.New("price must be greater than zero")
	ErrInvalidQuantity  = errors.New("quantity must be a non-negative number")
)

// ProductService defines business operations for the product catalog
type ProductService interface {
	CreateProduct(sku, name, description string, categoryID uint, price float64, stock int) (*models.Product, error)
	GetProductByID(id uint) (*models.Product, error)
	GetProductBySKU(sku string) (*models.Product, error)
	UpdateProduct(id uint, name, description string, categoryID uint, price float64, status models.ProductStatus) (*models.Product, error)
	DeactivateProduct(id uint) error
	ListProducts(filter repositories.ProductFilter) ([]models.ProductResponse, *response.Pagination, error)
}

type productService struct {
	prodRepo repositories.ProductRepository
	catRepo  repositories.CategoryRepository
	invRepo  repositories.InventoryRepository
}

// NewProductService returns an instance of ProductService
func NewProductService(
	prodRepo repositories.ProductRepository,
	catRepo repositories.CategoryRepository,
	invRepo repositories.InventoryRepository,
) ProductService {
	return &productService{
		prodRepo: prodRepo,
		catRepo:  catRepo,
		invRepo:  invRepo,
	}
}

func (s *productService) CreateProduct(sku, name, description string, categoryID uint, price float64, stock int) (*models.Product, error) {
	cleanSKU := strings.ToUpper(strings.TrimSpace(sku))
	cleanName := strings.TrimSpace(name)

	if cleanSKU == "" {
		return nil, errors.New("SKU is required")
	}
	if cleanName == "" {
		return nil, errors.New("product name is required")
	}
	if price <= 0 {
		return nil, ErrInvalidPrice
	}
	if stock < 0 {
		return nil, ErrInvalidQuantity
	}

	// Verify category exists
	category, err := s.catRepo.GetByID(categoryID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, ErrCategoryNotFound
	}

	// Verify SKU uniqueness
	existingSKU, err := s.prodRepo.GetBySKU(cleanSKU)
	if err != nil {
		return nil, err
	}
	if existingSKU != nil {
		return nil, ErrSKUAlreadyExists
	}

	product := &models.Product{
		SKU:           cleanSKU,
		Name:          cleanName,
		Description:   strings.TrimSpace(description),
		CategoryID:    categoryID,
		Price:         price,
		Stock:         stock,
		ReservedStock: 0,
		Status:        models.ProductStatusActive,
	}

	if err := s.prodRepo.Create(product); err != nil {
		return nil, err
	}

	// Record initial inventory transaction if stock > 0
	if stock > 0 {
		tx := &models.InventoryTransaction{
			ProductID:   product.ID,
			Type:        models.TxTypeStockIn,
			Quantity:    stock,
			ReferenceID: fmt.Sprintf("INIT-%d", product.ID),
			Notes:       "Initial product registration stock",
			CreatedAt:   time.Now().UTC(),
		}
		_ = s.invRepo.CreateTransaction(nil, tx)
	}

	// Reload with relations
	return s.prodRepo.GetByID(product.ID)
}

func (s *productService) GetProductByID(id uint) (*models.Product, error) {
	product, err := s.prodRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, repositories.ErrProductNotFound
	}
	return product, nil
}

func (s *productService) GetProductBySKU(sku string) (*models.Product, error) {
	product, err := s.prodRepo.GetBySKU(strings.ToUpper(strings.TrimSpace(sku)))
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, repositories.ErrProductNotFound
	}
	return product, nil
}

func (s *productService) UpdateProduct(id uint, name, description string, categoryID uint, price float64, status models.ProductStatus) (*models.Product, error) {
	product, err := s.prodRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, repositories.ErrProductNotFound
	}

	if strings.TrimSpace(name) != "" {
		product.Name = strings.TrimSpace(name)
	}
	if strings.TrimSpace(description) != "" {
		product.Description = strings.TrimSpace(description)
	}
	if categoryID > 0 && categoryID != product.CategoryID {
		cat, err := s.catRepo.GetByID(categoryID)
		if err != nil {
			return nil, err
		}
		if cat == nil {
			return nil, ErrCategoryNotFound
		}
		product.CategoryID = categoryID
	}
	if price > 0 {
		product.Price = price
	}
	if status != "" {
		product.Status = status
	}

	if err := s.prodRepo.Update(product); err != nil {
		return nil, err
	}

	return s.prodRepo.GetByID(id)
}

func (s *productService) DeactivateProduct(id uint) error {
	product, err := s.prodRepo.GetByID(id)
	if err != nil {
		return err
	}
	if product == nil {
		return repositories.ErrProductNotFound
	}
	return s.prodRepo.Delete(id)
}

func (s *productService) ListProducts(filter repositories.ProductFilter) ([]models.ProductResponse, *response.Pagination, error) {
	products, total, err := s.prodRepo.List(filter)
	if err != nil {
		return nil, nil, err
	}

	responses := make([]models.ProductResponse, len(products))
	for i := range products {
		responses[i] = products[i].ToResponse()
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages == 0 && total > 0 {
		totalPages = 1
	}

	pagination := &response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	return responses, pagination, nil
}
