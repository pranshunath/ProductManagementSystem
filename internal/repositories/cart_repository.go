package repositories

import (
	"errors"

	"producthub/internal/models"

	"gorm.io/gorm"
)

// CartRepository defines contract for user shopping cart persistence
type CartRepository interface {
	GetByUserID(userID uint) (*models.Cart, error)
	AddItem(userID uint, productID uint, quantity int) (*models.Cart, error)
	UpdateItemQuantity(userID uint, productID uint, quantity int) (*models.Cart, error)
	RemoveItem(userID uint, productID uint) (*models.Cart, error)
	ClearCart(userID uint) error
}

type cartRepository struct {
	db *gorm.DB
}

// NewCartRepository returns an instance of CartRepository
func NewCartRepository(db *gorm.DB) CartRepository {
	return &cartRepository{db: db}
}

func (r *cartRepository) GetByUserID(userID uint) (*models.Cart, error) {
	var cart models.Cart
	err := r.db.
		Preload("Items.Product.Category").
		Where("user_id = ?", userID).
		First(&cart).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Auto-create empty cart for user
			cart = models.Cart{UserID: userID}
			if errCreate := r.db.Create(&cart).Error; errCreate != nil {
				return nil, errCreate
			}
			return &cart, nil
		}
		return nil, err
	}

	return &cart, nil
}

func (r *cartRepository) AddItem(userID uint, productID uint, quantity int) (*models.Cart, error) {
	cart, err := r.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	var existingItem models.CartItem
	errFind := r.db.Where("cart_id = ? AND product_id = ?", cart.ID, productID).First(&existingItem).Error

	if errFind == nil {
		// Update quantity
		existingItem.Quantity += quantity
		if err := r.db.Save(&existingItem).Error; err != nil {
			return nil, err
		}
	} else if errors.Is(errFind, gorm.ErrRecordNotFound) {
		// Add new item
		newItem := models.CartItem{
			CartID:    cart.ID,
			ProductID: productID,
			Quantity:  quantity,
		}
		if err := r.db.Create(&newItem).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, errFind
	}

	return r.GetByUserID(userID)
}

func (r *cartRepository) UpdateItemQuantity(userID uint, productID uint, quantity int) (*models.Cart, error) {
	cart, err := r.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	if quantity <= 0 {
		return r.RemoveItem(userID, productID)
	}

	result := r.db.Model(&models.CartItem{}).
		Where("cart_id = ? AND product_id = ?", cart.ID, productID).
		Update("quantity", quantity)

	if result.Error != nil {
		return nil, result.Error
	}

	return r.GetByUserID(userID)
}

func (r *cartRepository) RemoveItem(userID uint, productID uint) (*models.Cart, error) {
	cart, err := r.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	errDelete := r.db.Where("cart_id = ? AND product_id = ?", cart.ID, productID).Delete(&models.CartItem{}).Error
	if errDelete != nil {
		return nil, errDelete
	}

	return r.GetByUserID(userID)
}

func (r *cartRepository) ClearCart(userID uint) error {
	cart, err := r.GetByUserID(userID)
	if err != nil {
		return err
	}

	return r.db.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error
}
