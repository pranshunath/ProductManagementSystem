package repositories

import (
	"errors"
	"fmt"
	"time"

	"producthub/internal/models"

	"gorm.io/gorm"
)

var (
	ErrInsufficientStock = errors.New("insufficient available stock")
	ErrProductNotFound   = errors.New("product not found")
)

// InventoryRepository handles stock audit records and atomic concurrency-safe operations
type InventoryRepository interface {
	CreateTransaction(tx *gorm.DB, transaction *models.InventoryTransaction) error
	GetProductTransactions(productID uint, page, limit int) ([]models.InventoryTransaction, int64, error)
	ListAllTransactions(page, limit int) ([]models.InventoryTransaction, int64, error)
	Restock(productID uint, quantity int, notes string) (*models.Product, error)
	ReserveStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error
	ReleaseStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error
	CommitReservedStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error
}

type inventoryRepository struct {
	db *gorm.DB
}

// NewInventoryRepository returns an instance of InventoryRepository
func NewInventoryRepository(db *gorm.DB) InventoryRepository {
	return &inventoryRepository{db: db}
}

func (r *inventoryRepository) CreateTransaction(tx *gorm.DB, transaction *models.InventoryTransaction) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	if transaction.CreatedAt.IsZero() {
		transaction.CreatedAt = time.Now().UTC()
	}
	return db.Create(transaction).Error
}

func (r *inventoryRepository) GetProductTransactions(productID uint, page, limit int) ([]models.InventoryTransaction, int64, error) {
	var txs []models.InventoryTransaction
	var total int64

	query := r.db.Model(&models.InventoryTransaction{}).Where("product_id = ?", productID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&txs).Error; err != nil {
		return nil, 0, err
	}

	return txs, total, nil
}

func (r *inventoryRepository) ListAllTransactions(page, limit int) ([]models.InventoryTransaction, int64, error) {
	var txs []models.InventoryTransaction
	var total int64

	query := r.db.Model(&models.InventoryTransaction{}).Preload("Product")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&txs).Error; err != nil {
		return nil, 0, err
	}

	return txs, total, nil
}

// Restock adds stock to a product and creates an audit transaction atomically
func (r *inventoryRepository) Restock(productID uint, quantity int, notes string) (*models.Product, error) {
	if quantity <= 0 {
		return nil, errors.New("restock quantity must be greater than zero")
	}

	var product models.Product

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Fetch product with row lock for update
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&product, productID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductNotFound
			}
			return err
		}

		// 2. Increment stock atomically
		res := tx.Model(&models.Product{}).
			Where("id = ?", productID).
			Update("stock", gorm.Expr("stock + ?", quantity))

		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrProductNotFound
		}

		// 3. Record audit trail
		invTx := models.InventoryTransaction{
			ProductID:   productID,
			Type:        models.TxTypeStockIn,
			Quantity:    quantity,
			ReferenceID: fmt.Sprintf("RESTOCK-%d", time.Now().Unix()),
			Notes:       notes,
			CreatedAt:   time.Now().UTC(),
		}
		if err := tx.Create(&invTx).Error; err != nil {
			return err
		}

		// Refresh updated product
		return tx.Preload("Category").First(&product, productID).Error
	})

	if err != nil {
		return nil, err
	}
	return &product, nil
}

// ReserveStockAtomic guarantees atomic reservation without race conditions.
// Uses an atomic WHERE condition: (stock - reserved_stock) >= quantity
func (r *inventoryRepository) ReserveStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error {
	db := r.db
	if tx != nil {
		db = tx
	}

	// Atomic update with affected rows verification
	result := db.Model(&models.Product{}).
		Where("id = ? AND (stock - reserved_stock) >= ?", productID, quantity).
		Update("reserved_stock", gorm.Expr("reserved_stock + ?", quantity))

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInsufficientStock
	}

	// Audit transaction
	invTx := models.InventoryTransaction{
		ProductID:   productID,
		Type:        models.TxTypeReserved,
		Quantity:    quantity,
		ReferenceID: refID,
		Notes:       "Stock reserved for pending order",
		CreatedAt:   time.Now().UTC(),
	}
	return db.Create(&invTx).Error
}

// ReleaseStockAtomic releases previously reserved stock back to available stock
func (r *inventoryRepository) ReleaseStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error {
	db := r.db
	if tx != nil {
		db = tx
	}

	result := db.Model(&models.Product{}).
		Where("id = ? AND reserved_stock >= ?", productID, quantity).
		Update("reserved_stock", gorm.Expr("reserved_stock - ?", quantity))

	if result.Error != nil {
		return result.Error
	}

	invTx := models.InventoryTransaction{
		ProductID:   productID,
		Type:        models.TxTypeReleased,
		Quantity:    quantity,
		ReferenceID: refID,
		Notes:       "Reserved stock released back to inventory",
		CreatedAt:   time.Now().UTC(),
	}
	return db.Create(&invTx).Error
}

// CommitReservedStockAtomic converts reserved stock into final deducted stock
func (r *inventoryRepository) CommitReservedStockAtomic(tx *gorm.DB, productID uint, quantity int, refID string) error {
	db := r.db
	if tx != nil {
		db = tx
	}

	result := db.Model(&models.Product{}).
		Where("id = ? AND reserved_stock >= ? AND stock >= ?", productID, quantity, quantity).
		Updates(map[string]interface{}{
			"stock":          gorm.Expr("stock - ?", quantity),
			"reserved_stock": gorm.Expr("reserved_stock - ?", quantity),
		})

	if result.Error != nil {
		return result.Error
	}

	invTx := models.InventoryTransaction{
		ProductID:   productID,
		Type:        models.TxTypeStockOut,
		Quantity:    quantity,
		ReferenceID: refID,
		Notes:       "Stock permanently deducted upon order confirmation",
		CreatedAt:   time.Now().UTC(),
	}
	return db.Create(&invTx).Error
}
