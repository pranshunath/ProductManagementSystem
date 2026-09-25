package tests

import (
	"testing"

	"producthub/internal/models"
)

func TestUserModel_PasswordHashing(t *testing.T) {
	user := models.User{
		Name:  "Test User",
		Email: "test@producthub.com",
		Role:  models.RoleCustomer,
	}

	rawPass := "SuperSecret123"
	if err := user.SetPassword(rawPass); err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if user.PasswordHash == "" || user.PasswordHash == rawPass {
		t.Errorf("Password was not properly hashed")
	}

	if !user.CheckPassword(rawPass) {
		t.Errorf("Expected password check to succeed for correct password")
	}

	if user.CheckPassword("WrongPassword") {
		t.Errorf("Expected password check to fail for incorrect password")
	}
}

func TestProductModel_AvailableStockAndStatus(t *testing.T) {
	p := models.Product{
		Name:          "Test Laptop",
		Stock:         20,
		ReservedStock: 5,
		Price:         999.99,
		Status:        models.ProductStatusActive,
	}

	if p.AvailableStock() != 15 {
		t.Errorf("Expected available stock 15, got %d", p.AvailableStock())
	}
	if p.StockStatus() != "IN_STOCK" {
		t.Errorf("Expected status IN_STOCK, got %s", p.StockStatus())
	}

	// Test Low Stock
	p.Stock = 8
	p.ReservedStock = 0
	if p.StockStatus() != "LOW_STOCK" {
		t.Errorf("Expected status LOW_STOCK, got %s", p.StockStatus())
	}

	// Test Out of Stock
	p.Stock = 5
	p.ReservedStock = 5
	if p.AvailableStock() != 0 {
		t.Errorf("Expected available stock 0, got %d", p.AvailableStock())
	}
	if p.StockStatus() != "OUT_OF_STOCK" {
		t.Errorf("Expected status OUT_OF_STOCK, got %s", p.StockStatus())
	}
}

func TestOrderLifecycle_StateTransitions(t *testing.T) {
	order := models.Order{
		Status: models.OrderStatusPending,
	}

	// Allowed: PENDING -> CONFIRMED
	if !order.CanTransitionTo(models.OrderStatusConfirmed) {
		t.Errorf("Expected PENDING -> CONFIRMED to be valid")
	}

	// Allowed: PENDING -> CANCELLED
	if !order.CanTransitionTo(models.OrderStatusCancelled) {
		t.Errorf("Expected PENDING -> CANCELLED to be valid")
	}

	// Forbidden: PENDING -> DELIVERED
	if order.CanTransitionTo(models.OrderStatusDelivered) {
		t.Errorf("Expected PENDING -> DELIVERED to be forbidden")
	}

	// Forbidden: DELIVERED -> PENDING
	order.Status = models.OrderStatusDelivered
	if err := order.ValidateTransition(models.OrderStatusPending); err == nil {
		t.Errorf("Expected DELIVERED -> PENDING to throw error")
	}

	// Forbidden: CANCELLED -> PENDING
	order.Status = models.OrderStatusCancelled
	if err := order.ValidateTransition(models.OrderStatusPending); err == nil {
		t.Errorf("Expected CANCELLED -> PENDING to throw error")
	}
}
