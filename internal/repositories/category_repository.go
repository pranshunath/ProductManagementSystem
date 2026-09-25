package repositories

import (
	"errors"

	"producthub/internal/models"

	"gorm.io/gorm"
)

// CategoryRepository defines contract for category persistence
type CategoryRepository interface {
	GetByID(id uint) (*models.Category, error)
	GetByName(name string) (*models.Category, error)
	Create(category *models.Category) error
	Update(category *models.Category) error
	Delete(id uint) error
	List() ([]models.Category, error)
}

type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository returns an instance of CategoryRepository
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) GetByID(id uint) (*models.Category, error) {
	var category models.Category
	if err := r.db.First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) GetByName(name string) (*models.Category, error) {
	var category models.Category
	if err := r.db.Where("name = ?", name).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) Create(category *models.Category) error {
	return r.db.Create(category).Error
}

func (r *categoryRepository) Update(category *models.Category) error {
	return r.db.Save(category).Error
}

func (r *categoryRepository) Delete(id uint) error {
	return r.db.Delete(&models.Category{}, id).Error
}

func (r *categoryRepository) List() ([]models.Category, error) {
	var categories []models.Category
	if err := r.db.Order("name ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}
