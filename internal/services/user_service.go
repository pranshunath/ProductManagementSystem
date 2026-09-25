package services

import (
	"errors"
	"math"
	"strings"

	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/pkg/response"
)

var (
	ErrEmailAlreadyExists = errors.New("an account with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
)

// UserService defines business logic for user management and authentication
type UserService interface {
	Register(name, email, password string, role models.UserRole) (*models.User, error)
	Authenticate(email, password string) (*models.User, error)
	GetUserByID(id uint) (*models.User, error)
	ListUsers(page, limit int) ([]models.User, *response.Pagination, error)
}

type userService struct {
	userRepo repositories.UserRepository
}

// NewUserService returns an instance of UserService
func NewUserService(userRepo repositories.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) Register(name, email, password string, role models.UserRole) (*models.User, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	cleanName := strings.TrimSpace(name)

	if cleanName == "" {
		return nil, errors.New("name is required")
	}
	if cleanEmail == "" {
		return nil, errors.New("email is required")
	}
	if len(password) < 6 {
		return nil, errors.New("password must be at least 6 characters")
	}

	existing, err := s.userRepo.GetByEmail(cleanEmail)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	assignedRole := models.RoleCustomer
	if role == models.RoleAdmin {
		assignedRole = models.RoleAdmin
	}

	user := &models.User{
		Name:  cleanName,
		Email: cleanEmail,
		Role:  assignedRole,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) Authenticate(email, password string) (*models.User, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	user, err := s.userRepo.GetByEmail(cleanEmail)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if !user.CheckPassword(password) {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

func (s *userService) GetUserByID(id uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *userService) ListUsers(page, limit int) ([]models.User, *response.Pagination, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	users, total, err := s.userRepo.List(page, limit)
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

	return users, pagination, nil
}
