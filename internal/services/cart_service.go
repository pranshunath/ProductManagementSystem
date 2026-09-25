package services

import (
	"errors"

	"producthub/internal/models"
	"producthub/internal/repositories"
)

// CartItemResponse represents formatted item details in cart response
type CartItemResponse struct {
	ProductID      uint    `json:"product_id"`
	SKU            string  `json:"sku"`
	Name           string  `json:"name"`
	UnitPrice      float64 `json:"unit_price"`
	Quantity       int     `json:"quantity"`
	Subtotal       float64 `json:"subtotal"`
	AvailableStock int     `json:"available_stock"`
}

// CartResponse represents the entire user cart with calculated totals
type CartResponse struct {
	CartID      uint               `json:"cart_id"`
	UserID      uint               `json:"user_id"`
	Items       []CartItemResponse `json:"items"`
	TotalItems  int                `json:"total_items"`
	TotalAmount float64            `json:"total_amount"`
}

// CartService defines shopping cart business operations
type CartService interface {
	GetCart(userID uint) (*CartResponse, error)
	AddItem(userID uint, productID uint, quantity int) (*CartResponse, error)
	UpdateQuantity(userID uint, productID uint, quantity int) (*CartResponse, error)
	RemoveItem(userID uint, productID uint) (*CartResponse, error)
	ClearCart(userID uint) error
}

type cartService struct {
	cartRepo repositories.CartRepository
	prodRepo repositories.ProductRepository
}

// NewCartService returns an instance of CartService
func NewCartService(cartRepo repositories.CartRepository, prodRepo repositories.ProductRepository) CartService {
	return &cartService{
		cartRepo: cartRepo,
		prodRepo: prodRepo,
	}
}

func (s *cartService) formatCart(cart *models.Cart) *CartResponse {
	resp := &CartResponse{
		CartID: cart.ID,
		UserID: cart.UserID,
		Items:  make([]CartItemResponse, 0, len(cart.Items)),
	}

	for _, item := range cart.Items {
		unitPrice := item.Product.Price
		subtotal := unitPrice * float64(item.Quantity)
		resp.TotalItems += item.Quantity
		resp.TotalAmount += subtotal

		resp.Items = append(resp.Items, CartItemResponse{
			ProductID:      item.ProductID,
			SKU:            item.Product.SKU,
			Name:           item.Product.Name,
			UnitPrice:      unitPrice,
			Quantity:       item.Quantity,
			Subtotal:       subtotal,
			AvailableStock: item.Product.AvailableStock(),
		})
	}

	return resp
}

func (s *cartService) GetCart(userID uint) (*CartResponse, error) {
	cart, err := s.cartRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}
	return s.formatCart(cart), nil
}

func (s *cartService) AddItem(userID uint, productID uint, quantity int) (*CartResponse, error) {
	if quantity <= 0 {
		return nil, errors.New("quantity must be greater than zero")
	}

	product, err := s.prodRepo.GetByID(productID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, repositories.ErrProductNotFound
	}
	if product.Status != models.ProductStatusActive {
		return nil, errors.New("product is not available for purchase")
	}

	// Validate available stock
	available := product.AvailableStock()
	if available < quantity {
		return nil, repositories.ErrInsufficientStock
	}

	cart, err := s.cartRepo.AddItem(userID, productID, quantity)
	if err != nil {
		return nil, err
	}
	return s.formatCart(cart), nil
}

func (s *cartService) UpdateQuantity(userID uint, productID uint, quantity int) (*CartResponse, error) {
	if quantity < 0 {
		return nil, errors.New("quantity cannot be negative")
	}

	if quantity > 0 {
		product, err := s.prodRepo.GetByID(productID)
		if err != nil {
			return nil, err
		}
		if product == nil {
			return nil, repositories.ErrProductNotFound
		}
		if product.AvailableStock() < quantity {
			return nil, repositories.ErrInsufficientStock
		}
	}

	cart, err := s.cartRepo.UpdateItemQuantity(userID, productID, quantity)
	if err != nil {
		return nil, err
	}
	return s.formatCart(cart), nil
}

func (s *cartService) RemoveItem(userID uint, productID uint) (*CartResponse, error) {
	cart, err := s.cartRepo.RemoveItem(userID, productID)
	if err != nil {
		return nil, err
	}
	return s.formatCart(cart), nil
}

func (s *cartService) ClearCart(userID uint) error {
	return s.cartRepo.ClearCart(userID)
}
