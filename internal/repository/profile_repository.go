// internal/repository/profile_repository.go
package repository

import (
	"context"
	"dompetku/internal/model"
	"os"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProfileRepository menangani semua operasi DB yang berkaitan dengan profil user.
// Operasi baca (FindByID) sudah ada di UserRepository — di sini kita tambah
// operasi tulis khusus profil agar UserRepository tetap fokus pada auth.
type ProfileRepository interface {
	UpdateProfile(ctx context.Context, id uuid.UUID, name, email string) (*model.User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error
	UpdateAvatar(ctx context.Context, id uuid.UUID, avatarURL string) (*model.User, error)
	DeleteAccount(ctx context.Context, id uuid.UUID) error

	// Statistik
	SumWalletBalance(ctx context.Context, userID uuid.UUID) (float64, error)
	CountTransactions(ctx context.Context, userID uuid.UUID) (int64, error)
	CountWallets(ctx context.Context, userID uuid.UUID) (int64, error)

	// Cek duplikat email (selain milik sendiri)
	EmailExistsExcept(ctx context.Context, email string, excludeID uuid.UUID) (bool, error)
}

type profileRepository struct {
	db *gorm.DB
}

func NewProfileRepository(db *gorm.DB) ProfileRepository {
	return &profileRepository{db: db}
}

func (r *profileRepository) UpdateProfile(ctx context.Context, id uuid.UUID, name, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Model(&user).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":  name,
			"email": email,
		}).Error
	if err != nil {
		return nil, err
	}

	// Kembalikan data terbaru
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *profileRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Update("password", hashedPassword).Error
}

func (r *profileRepository) UpdateAvatar(ctx context.Context, id uuid.UUID, avatarURL string) (*model.User, error) {
	// Ambil avatar lama untuk dihapus dari disk setelah update
	var old model.User
	r.db.WithContext(ctx).Select("avatar_url").First(&old, "id = ?", id)

	var user model.User
	err := r.db.WithContext(ctx).
		Model(&user).
		Where("id = ?", id).
		Update("avatar_url", avatarURL).Error
	if err != nil {
		return nil, err
	}

	// Hapus file lama dari disk (abaikan error jika file tidak ada)
	if old.AvatarURL != "" && strings.HasPrefix(old.AvatarURL, "/uploads/") {
		os.Remove("." + old.AvatarURL)
	}

	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *profileRepository) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	// Soft delete — GORM mengisi deleted_at secara otomatis.
	// Semua relasi (Wallet, Transaction, dst) ikut terhapus karena
	// constraint ON DELETE CASCADE di masing-masing model.
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.User{}).Error
}

func (r *profileRepository) SumWalletBalance(ctx context.Context, userID uuid.UUID) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).
		Model(&model.Wallet{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Select("COALESCE(SUM(balance), 0)").
		Scan(&total).Error
	return total, err
}

func (r *profileRepository) CountTransactions(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

func (r *profileRepository) CountWallets(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Wallet{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

func (r *profileRepository) EmailExistsExcept(ctx context.Context, email string, excludeID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("email = ? AND id != ? AND deleted_at IS NULL", email, excludeID).
		Count(&count).Error
	return count > 0, err
}