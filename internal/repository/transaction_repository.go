package repository

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransactionRepository interface {
	Create(transaction *model.Transaction) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Transaction, error)
	FindAll(userID uuid.UUID, filter dto.TransactionFilterRequest) ([]model.Transaction, error)
	Update(transaction *model.Transaction) error
	Delete(id uuid.UUID, userID uuid.UUID) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Wallet & kategori yang sudah soft-delete tetap dimuat agar namanya tidak kosong
// di riwayat transaksi.
func withRelations(db *gorm.DB, userID uuid.UUID) *gorm.DB {
	return db.
		Preload("Wallet", func(tx *gorm.DB) *gorm.DB {
			return tx.Unscoped().Where("user_id = ?", userID)
		}).
		Preload("Category", func(tx *gorm.DB) *gorm.DB {
			return tx.Unscoped().Where("user_id = ?", userID)
		})
}

func (r *transactionRepository) Create(transaction *model.Transaction) error {
	return r.db.Omit(clause.Associations).Create(transaction).Error
}

func (r *transactionRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Transaction, error) {
	var transaction model.Transaction

	err := withRelations(r.db, userID).
		Where("id = ? AND user_id = ?", id, userID).
		First(&transaction).Error
	if err != nil {
		return nil, err
	}

	return &transaction, nil
}

func (r *transactionRepository) FindAll(userID uuid.UUID, filter dto.TransactionFilterRequest) ([]model.Transaction, error) {
	var transactions []model.Transaction

	query := withRelations(r.db, userID).Where("user_id = ?", userID)

	if filter.WalletID != "" {
		query = query.Where("wallet_id = ?", filter.WalletID)
	}
	if filter.CategoryID != "" {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.StartDate != nil {
		query = query.Where("transaction_date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("transaction_date <= ?", *filter.EndDate)
	}

	err := query.Order("transaction_date DESC, created_at DESC").Find(&transactions).Error
	return transactions, err
}

// Omit(clause.Associations) wajib: tanpa itu GORM menimpa wallet_id/category_id
// baru dengan ID dari struct Wallet/Category yang sudah di-preload.
func (r *transactionRepository) Update(transaction *model.Transaction) error {
	return r.db.Omit(clause.Associations).Save(transaction).Error
}

func (r *transactionRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	return r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Transaction{}).Error
}
