package repositories

import (
	"errors"
	"fmt"
	"strings"

	"producthub/internal/models"

	"gorm.io/gorm"
)

// ProductFilter encapsulates query parameters for listing products
type ProductFilter struct {
	Page         int
	Limit        int
	Search       string
	Category     string
	CategoryID   uint
	MinPrice     *float64
	MaxPrice     *float64
	Status       string
	Sort         string
	Order        string
	IncludeEmpty bool
}

// ProductRepository defines contract for product catalog persistence
type ProductRepository interface {
	GetByID(id uint) (*models.Product, error)
	GetBySKU(sku string) (*models.Product, error)
	Create(product *models.Product) error
	Update(product *models.Product) error
	Delete(id uint) error
	List(filter ProductFilter) ([]models.Product, int64, error)
	CountTotal() (int64, error)
	CountLowStock(threshold int) (int64, error)
	CountOutOfStock() (int64, error)
	GetLowStockProducts(threshold int, limit int) ([]models.Product, error)
}

type productRepository struct {
	db *gorm.DB
}

// NewProductRepository returns an instance of ProductRepository
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) GetByID(id uint) (*models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Category").First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) GetBySKU(sku string) (*models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Category").Where("sku = ?", sku).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

func (r *productRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}

// Delete logically deactivates or soft-deletes the product
func (r *productRepository) Delete(id uint) error {
	return r.db.Model(&models.Product{}).Where("id = ?", id).Update("status", models.ProductStatusInactive).Error
}

func (r *productRepository) List(filter ProductFilter) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{}).Preload("Category")

	// Filter: Search across Name, Description, SKU
	if strings.TrimSpace(filter.Search) != "" {
		searchTerm := "%" + strings.TrimSpace(filter.Search) + "%"
		query = query.Where("name LIKE ? OR description LIKE ? OR sku LIKE ?", searchTerm, searchTerm, searchTerm)
	}

	// Filter: Category ID
	if filter.CategoryID > 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	} else if strings.TrimSpace(filter.Category) != "" {
		query = query.Joins("JOIN categories ON categories.id = products.category_id").
			Where("LOWER(categories.name) = ?", strings.ToLower(strings.TrimSpace(filter.Category)))
	}

	// Filter: Price Range
	if filter.MinPrice != nil {
		query = query.Where("price >= ?", *filter.MinPrice)
	}
	if filter.MaxPrice != nil {
		query = query.Where("price <= ?", *filter.MaxPrice)
	}

	// Filter: Status (defaults to ACTIVE unless explicitly requested or empty)
	if filter.Status != "" {
		query = query.Where("products.status = ?", filter.Status)
	}

	// Count total matching records before pagination
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sorting with strict whitelist to prevent SQL injection
	sortColumn := "products.id"
	switch strings.ToLower(filter.Sort) {
	case "price":
		sortColumn = "products.price"
	case "name":
		sortColumn = "products.name"
	case "created_at":
		sortColumn = "products.created_at"
	case "stock":
		sortColumn = "products.stock"
	}

	sortOrder := "DESC"
	if strings.ToLower(filter.Order) == "asc" {
		sortOrder = "ASC"
	}

	orderClause := fmt.Sprintf("%s %s", sortColumn, sortOrder)

	// Pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 10
	}
	offset := (page - 1) * limit

	if err := query.Order(orderClause).Offset(offset).Limit(limit).Find(&products).Error; err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (r *productRepository) CountTotal() (int64, error) {
	var count int64
	err := r.db.Model(&models.Product{}).Count(&count).Error
	return count, err
}

func (r *productRepository) CountLowStock(threshold int) (int64, error) {
	var count int64
	err := r.db.Model(&models.Product{}).
		Where("status = ? AND (stock - reserved_stock) > 0 AND (stock - reserved_stock) <= ?", models.ProductStatusActive, threshold).
		Count(&count).Error
	return count, err
}

func (r *productRepository) CountOutOfStock() (int64, error) {
	var count int64
	err := r.db.Model(&models.Product{}).
		Where("status = ? AND (stock - reserved_stock) <= 0", models.ProductStatusActive).
		Count(&count).Error
	return count, err
}

func (r *productRepository) GetLowStockProducts(threshold int, limit int) ([]models.Product, error) {
	var products []models.Product
	err := r.db.Preload("Category").
		Where("status = ? AND (stock - reserved_stock) <= ?", models.ProductStatusActive, threshold).
		Order("(stock - reserved_stock) ASC").
		Limit(limit).
		Find(&products).Error
	return products, err
}
