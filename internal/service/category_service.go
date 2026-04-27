package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CategoryService interface {
	CreateCategory(userID uuid.UUID, req dto.CreateCategoryRequest) (*dto.CategoryResponse, error)
	GetCategoryByID(userID uuid.UUID, id uuid.UUID) (*dto.CategoryResponse, error)
	GetAllCategories(userID uuid.UUID, categoryType *string) ([]dto.CategoryResponse, error)
	UpdateCategory(userID uuid.UUID, id uuid.UUID, req dto.UpdateCategoryRequest) (*dto.CategoryResponse, error)
	DeleteCategory(userID uuid.UUID, id uuid.UUID) error
}

type categoryService struct {
	categoryRepo repository.CategoryRepository
}

func NewCategoryService(categoryRepo repository.CategoryRepository) CategoryService {
	return &categoryService{categoryRepo: categoryRepo}
}

func (s *categoryService) CreateCategory(userID uuid.UUID, req dto.CreateCategoryRequest) (*dto.CategoryResponse, error) {
	category := &model.Category{
		UserID: userID,
		Name:   req.Name,
		Type:   req.Type,
	}

	if err := s.categoryRepo.Create(category); err != nil {
		return nil, err
	}

	return toResponse(category), nil
}

func (s *categoryService) GetCategoryByID(userID uuid.UUID, id uuid.UUID) (*dto.CategoryResponse, error) {
	category, err := s.categoryRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("kategori tidak ditemukan")
		}
		return nil, err
	}
	return toResponse(category), nil
}

func (s *categoryService) GetAllCategories(userID uuid.UUID, categoryType *string) ([]dto.CategoryResponse, error) {
	categories, err := s.categoryRepo.FindAll(userID, categoryType)
	if err != nil {
		return nil, err
	}

	var responses []dto.CategoryResponse
	for _, c := range categories {
		responses = append(responses, *toResponse(&c))
	}

	if responses == nil {
		responses = []dto.CategoryResponse{}
	}

	return responses, nil
}

func (s *categoryService) UpdateCategory(userID uuid.UUID, id uuid.UUID, req dto.UpdateCategoryRequest) (*dto.CategoryResponse, error) {
	category, err := s.categoryRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("kategori tidak ditemukan")
		}
		return nil, err
	}

	// Partial update — hanya update field yang dikirim
	if req.Name != nil {
		category.Name = *req.Name
	}
	if req.Type != nil {
		category.Type = *req.Type
	}

	if err := s.categoryRepo.Update(category); err != nil {
		return nil, err
	}

	return toResponse(category), nil
}

func (s *categoryService) DeleteCategory(userID uuid.UUID, id uuid.UUID) error {
	_, err := s.categoryRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("kategori tidak ditemukan")
		}
		return err
	}
	return s.categoryRepo.Delete(id, userID)
}

// ── Helper ────────────────────────────────────────────────────────────

func toResponse(c *model.Category) *dto.CategoryResponse {
	return &dto.CategoryResponse{
		ID:        c.ID,
		UserID:    c.UserID,
		Name:      c.Name,
		Type:      c.Type,
		CreatedAt: c.CreatedAt,
	}
}