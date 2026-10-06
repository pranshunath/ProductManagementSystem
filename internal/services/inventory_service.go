package services

import (
	"errors"
	"math"

	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/pkg/response"
)

// InventoryService defines business logic for stock management and audit trails
type InventoryService interface {
	RestockProduct(productID uint, quantity int, notes string) (*models.Product, error)
	CheckStock(productID uint, requestedQuantity int) (bool, int, error)
	GetProductAuditTrail(productID uint, page, limit int) ([]models.InventoryTransaction, *response.Pagination, error)
	ListAllAuditTrails(page, limit int) ([]models.InventoryTransaction, *response.Pagination, error)
	GetInventorySummary() (map[string]interface{}, error)
}

type inventoryService struct {
	invRepo  repositories.InventoryRepository
	prodRepo repositories.ProductRepository
}

// NewInventoryService returns an instance of InventoryService
func NewInventoryService(
	invRepo repositories.InventoryRepository,
	prodRepo repositories.ProductRepository,
) InventoryService {
	return &inventoryService{
		invRepo:  invRepo,
		prodRepo: prodRepo,
	}
}

func (s *inventoryService) RestockProduct(productID uint, quantity int, notes string) (*models.Product, error) {
	if quantity <= 0 {
		return nil, errors.New("quantity must be greater than zero")
	}

	prod, err := s.prodRepo.GetByID(productID)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, repositories.ErrProductNotFound
	}

	return s.invRepo.Restock(productID, quantity, notes)
}

func (s *inventoryService) CheckStock(productID uint, requestedQuantity int) (bool, int, error) {
	prod, err := s.prodRepo.GetByID(productID)
	if err != nil {
		return false, 0, err
	}
	if prod == nil {
		return false, 0, repositories.ErrProductNotFound
	}

	avail := prod.AvailableStock()
	canFulfill := avail >= requestedQuantity && requestedQuantity > 0
	return canFulfill, avail, nil
}

func (s *inventoryService) GetProductAuditTrail(productID uint, page, limit int) ([]models.InventoryTransaction, *response.Pagination, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	txs, total, err := s.invRepo.GetProductTransactions(productID, page, limit)
	if err != nil {
		return nil, nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	pagination := &response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	return txs, pagination, nil
}

func (s *inventoryService) ListAllAuditTrails(page, limit int) ([]models.InventoryTransaction, *response.Pagination, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	txs, total, err := s.invRepo.ListAllTransactions(page, limit)
	if err != nil {
		return nil, nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	pagination := &response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	return txs, pagination, nil
}

func (s *inventoryService) GetInventorySummary() (map[string]interface{}, error) {
	totalProducts, err := s.prodRepo.CountTotal()
	if err != nil {
		return nil, err
	}

	totalStock, err := s.prodRepo.SumStock()
	if err != nil {
		return nil, err
	}

	lowStock, err := s.prodRepo.CountLowStock(10)
	if err != nil {
		return nil, err
	}

	outOfStock, err := s.prodRepo.CountOutOfStock()
	if err != nil {
		return nil, err
	}

	lowStockItems, err := s.prodRepo.GetLowStockProducts(10, 5)
	if err != nil {
		return nil, err
	}

	lowStockResponses := make([]models.ProductResponse, len(lowStockItems))
	for i := range lowStockItems {
		lowStockResponses[i] = lowStockItems[i].ToResponse()
	}

	return map[string]interface{}{
		"total_products":     totalProducts,
		"total_stock":        totalStock,
		"low_stock_count":    lowStock,
		"out_of_stock_count": outOfStock,
		"low_stock_items":    lowStockResponses,
	}, nil
}
