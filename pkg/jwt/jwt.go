package jwt

import (
	"errors"
	"fmt"
	"time"

	"producthub/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or malformed token")
	ErrExpiredToken = errors.New("token has expired")
)

// CustomClaims represents the JWT claims payload
type CustomClaims struct {
	UserID uint            `json:"user_id"`
	Email  string          `json:"email"`
	Role   models.UserRole `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken generates a signed HMAC-SHA256 JWT string for a user
func GenerateToken(userID uint, email string, role models.UserRole, secret string, expirationHours int) (string, error) {
	if secret == "" {
		return "", errors.New("JWT secret cannot be empty")
	}

	claims := CustomClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expirationHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "producthub-api",
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses, validates signature, and extracts claims from a JWT string
func ValidateToken(tokenString string, secret string) (*CustomClaims, error) {
	if secret == "" {
		return nil, errors.New("JWT secret cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure token was signed with expected HMAC method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
