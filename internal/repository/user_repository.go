package repository

import (
	"context"
	"dompetku/internal/model"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	// EmailTaken ikut menghitung akun yang sudah soft-delete karena unique index
	// email di database masih memegang baris tersebut.
	EmailTaken(ctx context.Context, email string) (bool, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User

	err := r.db.WithContext(ctx).
		Where("LOWER(email) = LOWER(?)", email).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (r *userRepository) EmailTaken(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Unscoped().
		Model(&model.User{}).
		Where("LOWER(email) = LOWER(?)", email).
		Count(&count).Error
	return count > 0, err
}
