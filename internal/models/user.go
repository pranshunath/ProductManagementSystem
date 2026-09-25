package models

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserRole represents system authorization roles
type UserRole string

const (
	RoleCustomer UserRole = "CUSTOMER"
	RoleAdmin    UserRole = "ADMIN"
)

// User represents an authenticated account in the system
type User struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	Email        string         `gorm:"size:150;uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Role         UserRole       `gorm:"size:20;not null;default:'CUSTOMER';index" json:"role"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// SetPassword hashes and updates the user's password
func (u *User) SetPassword(rawPassword string) error {
	if len(rawPassword) < 6 {
		return errors.New("password must be at least 6 characters long")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword compares raw password with the hashed password
func (u *User) CheckPassword(rawPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(rawPassword))
	return err == nil
}

// IsAdmin returns true if the user has an ADMIN role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
