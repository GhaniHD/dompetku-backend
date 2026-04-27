package repository

import (
	"dompetku/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CategoryRepository interface {
	Create(category *model.Category) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Category, error)
	FindAll(userID uuid.UUID, categoryType *string) ([]model.Category, error)
	Update(category *model.Category) error
	Delete(id uuid.UUID, userID uuid.UUID) error
}

type categoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(category *model.Category) error {
	return r.db.Create(category).Error
}

func (r *categoryRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Category, error) {
	var category model.Category
	err := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) FindAll(userID uuid.UUID, categoryType *string) ([]model.Category, error) {
	var categories []model.Category
	query := r.db.Where("user_id = ?", userID)
	if categoryType != nil {
		query = query.Where("type = ?", *categoryType)
	}
	err := query.Order("created_at DESC").Find(&categories).Error
	return categories, err
}

func (r *categoryRepository) Update(category *model.Category) error {
	return r.db.Save(category).Error
}

func (r *categoryRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	return r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Category{}).Error
}