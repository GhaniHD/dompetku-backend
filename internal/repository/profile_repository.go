// internal/repository/profile_repository.go
package repository

import (
	"context"
	"dompetku/internal/model"
	"os"
	"strings"
	"time"

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
	UpdateBudgetStartDay(ctx context.Context, id uuid.UUID, day int) error
	UpdateIncome(ctx context.Context, id uuid.UUID, incomeType string, day int) error
	UpdateDailyCategories(ctx context.Context, id uuid.UUID, categoryIDs []uuid.UUID) error
	CountExpenseCategories(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (int64, error)

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
		Updates(map[string]any{"password": hashedPassword, "token_version": gorm.Expr("token_version + 1"), "session_reason": model.SessionPassword}).Error
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

// DeleteAccount menghapus permanen akun beserta seluruh datanya.
func (r *profileRepository) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return PurgeUser(tx, id)
	})
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
		Unscoped().
		Model(&model.User{}).
		Where("LOWER(email) = LOWER(?) AND id != ?", email, excludeID).
		Count(&count).Error
	return count > 0, err
}
func (r *profileRepository) UpdateIncome(ctx context.Context, id uuid.UUID, incomeType string, day int) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"income_type": incomeType, "budget_start_day": day, "income_setup_at": time.Now()}).Error
}

func (r *profileRepository) UpdateDailyCategories(ctx context.Context, id uuid.UUID, categoryIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.User{ID: id}).
		Select("daily_category_ids").
		Updates(&model.User{DailyCategoryIDs: categoryIDs}).Error
}

func (r *profileRepository) CountExpenseCategories(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("user_id = ? AND type = ? AND id IN ?", userID, "expense", ids).
		Count(&n).Error
	return n, err
}

func (r *profileRepository) UpdateBudgetStartDay(ctx context.Context, id uuid.UUID, day int) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Update("budget_start_day", day).Error
}
