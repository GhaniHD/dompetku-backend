package repository

import (
	"dompetku/internal/model"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrWalletNotFound = errors.New("dompet tidak ditemukan")

type WalletRepository interface {
	Create(wallet *model.Wallet) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Wallet, error)
	// FindByIDForUpdate mengunci baris wallet (SELECT ... FOR UPDATE) dan ikut
	// mengembalikan wallet yang sudah soft-delete; cek DeletedAt di pemanggil.
	// Wajib dipanggil di dalam db.Transaction.
	FindByIDForUpdate(id uuid.UUID, userID uuid.UUID) (*model.Wallet, error)
	FindAll(userID uuid.UUID, includeArchived bool) ([]model.Wallet, error)
	Update(wallet *model.Wallet) error
	Delete(id uuid.UUID, userID uuid.UUID) error
	SumBalance(userID uuid.UUID) (float64, error)
	AddBalance(walletID uuid.UUID, userID uuid.UUID, delta float64) error
	HasHistory(walletID uuid.UUID) (bool, error)
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
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) FindByIDForUpdate(id uuid.UUID, userID uuid.UUID) (*model.Wallet, error) {
	var wallet model.Wallet
	err := r.db.
		Unscoped().
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", id, userID).
		First(&wallet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) FindAll(userID uuid.UUID, includeArchived bool) ([]model.Wallet, error) {
	var wallets []model.Wallet
	query := r.db.Where("user_id = ?", userID)
	if !includeArchived {
		query = query.Where("is_archived = false")
	}
	err := query.Order("is_archived ASC, created_at DESC").Find(&wallets).Error
	return wallets, err
}

func (r *walletRepository) Update(wallet *model.Wallet) error {
	return r.db.Omit(clause.Associations).Save(wallet).Error
}

func (r *walletRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	result := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Wallet{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWalletNotFound
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

// AddBalance juga berlaku untuk wallet yang sudah soft-delete, supaya hapus/edit
// transaksi lama tetap bisa mengembalikan saldonya.
func (r *walletRepository) AddBalance(walletID uuid.UUID, userID uuid.UUID, delta float64) error {
	result := r.db.Unscoped().Model(&model.Wallet{}).
		Where("id = ? AND user_id = ?", walletID, userID).
		UpdateColumn("balance", gorm.Expr("balance + ?", delta))

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWalletNotFound
	}
	return nil
}

// HasHistory bernilai true bila wallet pernah dipakai transaksi, transfer, atau
// penyesuaian saldo; wallet seperti ini diarsipkan, bukan dihapus.
func (r *walletRepository) HasHistory(walletID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.Raw(`SELECT
		EXISTS (SELECT 1 FROM transactions WHERE wallet_id = ? AND deleted_at IS NULL)
		OR EXISTS (SELECT 1 FROM transfers WHERE (from_wallet_id = ? OR to_wallet_id = ?) AND deleted_at IS NULL)
		OR EXISTS (SELECT 1 FROM balance_adjustments WHERE wallet_id = ? AND deleted_at IS NULL)`,
		walletID, walletID, walletID, walletID).Scan(&exists).Error
	return exists, err
}
