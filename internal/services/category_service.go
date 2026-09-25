package services

import (
	"errors"
	"strings"

	"producthub/internal/models"
	"producthub/internal/repositories"
)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryNameUsed = errors.New("category name already in use")
)

// CategoryService defines business logic for product categories
type CategoryService interface {
	CreateCategory(name, description string) (*models.Category, error)
	GetCategory(id uint) (*models.Category, error)
	ListCategories() ([]models.Category, error)
	UpdateCategory(id uint, name, description string) (*models.Category, error)
	DeleteCategory(id uint) error
}

type categoryService struct {
	catRepo repositories.CategoryRepository
}

// NewCategoryService returns an instance of CategoryService
func NewCategoryService(catRepo repositories.CategoryRepository) CategoryService {
	return &categoryService{catRepo: catRepo}
}

func (s *categoryService) CreateCategory(name, description string) (*models.Category, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, errors.New("category name is required")
	}

	existing, err := s.catRepo.GetByName(cleanName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrCategoryNameUsed
	}

	cat := &models.Category{
		Name:        cleanName,
		Description: strings.TrimSpace(description),
	}

	if err := s.catRepo.Create(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *categoryService) GetCategory(id uint) (*models.Category, error) {
	cat, err := s.catRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, ErrCategoryNotFound
	}
	return cat, nil
}

func (s *categoryService) ListCategories() ([]models.Category, error) {
	return s.catRepo.List()
}

func (s *categoryService) UpdateCategory(id uint, name, description string) (*models.Category, error) {
	cat, err := s.catRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, ErrCategoryNotFound
	}

	cleanName := strings.TrimSpace(name)
	if cleanName != "" && cleanName != cat.Name {
		existing, err := s.catRepo.GetByName(cleanName)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != id {
			return nil, ErrCategoryNameUsed
		}
		cat.Name = cleanName
	}

	cat.Description = strings.TrimSpace(description)
	if err := s.catRepo.Update(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *categoryService) DeleteCategory(id uint) error {
	cat, err := s.catRepo.GetByID(id)
	if err != nil {
		return err
	}
	if cat == nil {
		return ErrCategoryNotFound
	}
	return s.catRepo.Delete(id)
}
