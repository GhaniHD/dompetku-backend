// internal/repository/wallet_repository.go
package repository

import (
	"dompetku/internal/model"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WalletRepository interface {
	Create(wallet *model.Wallet) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Wallet, error)
	FindAll(userID uuid.UUID) ([]model.Wallet, error)
	Update(wallet *model.Wallet) error
	Delete(id uuid.UUID, userID uuid.UUID) error
	SumBalance(userID uuid.UUID) (float64, error)
	// AddBalance menambah atau mengurangi balance secara atomic di DB
	// delta positif = tambah, delta negatif = kurangi
	AddBalance(walletID uuid.UUID, userID uuid.UUID, delta float64) error
}

type walletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) Create(wallet *model.Wallet) error {
	return r.db.Create(wallet).Error
}

func (r *walletRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Wallet, error) {
	var wallet model.Wallet
	err := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		First(&wallet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("dompet tidak ditemukan")
	}
	return &wallet, err
}

func (r *walletRepository) FindAll(userID uuid.UUID) ([]model.Wallet, error) {
	var wallets []model.Wallet
	err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&wallets).Error
	return wallets, err
}

func (r *walletRepository) Update(wallet *model.Wallet) error {
	return r.db.Save(wallet).Error
}

func (r *walletRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	result := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Wallet{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("dompet tidak ditemukan")
	}
	return nil
}

func (r *walletRepository) SumBalance(userID uuid.UUID) (float64, error) {
	var total float64
	err := r.db.Model(&model.Wallet{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Select("COALESCE(SUM(balance), 0)").
		Scan(&total).Error
	return total, err
}

// AddBalance mengupdate balance wallet secara atomic menggunakan SQL expression.
// Lebih aman dari read-then-write karena menghindari race condition.
func (r *walletRepository) AddBalance(walletID uuid.UUID, userID uuid.UUID, delta float64) error {
	result := r.db.Model(&model.Wallet{}).
		Where("id = ? AND user_id = ?", walletID, userID).
		UpdateColumn("balance", gorm.Expr("balance + ?", delta))

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("dompet tidak ditemukan")
	}
	return nil
}