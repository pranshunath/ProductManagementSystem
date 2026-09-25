package repositories

import (
	"errors"

	"producthub/internal/models"

	"gorm.io/gorm"
)

// OrderRepository handles order persistence and analytics queries
type OrderRepository interface {
	GetDB() *gorm.DB
	GetByID(id uint) (*models.Order, error)
	Create(tx *gorm.DB, order *models.Order) error
	UpdateStatus(tx *gorm.DB, id uint, status models.OrderStatus) error
	ListByUserID(userID uint, page, limit int) ([]models.Order, int64, error)
	ListAll(page, limit int, status models.OrderStatus) ([]models.Order, int64, error)
	CountTotal() (int64, error)
	CountByStatus(status models.OrderStatus) (int64, error)
	TotalRevenue() (float64, error)
	GetRecentOrders(limit int) ([]models.Order, error)
}

type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository returns an instance of OrderRepository
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *orderRepository) GetByID(id uint) (*models.Order, error) {
	var order models.Order
	err := r.db.
		Preload("Items.Product.Category").
		Preload("User").
		First(&order, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) Create(tx *gorm.DB, order *models.Order) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(order).Error
}

func (r *orderRepository) UpdateStatus(tx *gorm.DB, id uint, status models.OrderStatus) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Model(&models.Order{}).Where("id = ?", id).Update("status", status).Error
}

func (r *orderRepository) ListByUserID(userID uint, page, limit int) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := r.db.Model(&models.Order{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Items.Product").
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&orders).Error

	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *orderRepository) ListAll(page, limit int, status models.OrderStatus) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := r.db.Model(&models.Order{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Items.Product").
		Preload("User").
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&orders).Error

	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *orderRepository) CountTotal() (int64, error) {
	var count int64
	err := r.db.Model(&models.Order{}).Count(&count).Error
	return count, err
}

func (r *orderRepository) CountByStatus(status models.OrderStatus) (int64, error) {
	var count int64
	err := r.db.Model(&models.Order{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *orderRepository) TotalRevenue() (float64, error) {
	var total sqlNullFloat
	err := r.db.Model(&models.Order{}).
		Where("status NOT IN (?)", []models.OrderStatus{models.OrderStatusCancelled}).
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&total.Float64).Error

	return total.Float64, err
}

func (r *orderRepository) GetRecentOrders(limit int) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.
		Preload("Items.Product").
		Preload("User").
		Order("id DESC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

type sqlNullFloat struct {
	Float64 float64
}
