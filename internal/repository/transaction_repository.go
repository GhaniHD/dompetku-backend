package repository

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TransactionRepository mendefinisikan kontrak operasi database untuk transaksi
type TransactionRepository interface {
	Create(transaction *model.Transaction) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Transaction, error)
	FindAll(userID uuid.UUID, filter dto.TransactionFilterRequest) ([]model.Transaction, error)
	Update(transaction *model.Transaction) error
	Delete(id uuid.UUID, userID uuid.UUID) error
}

// transactionRepository adalah implementasi dari TransactionRepository
type transactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository membuat instance baru TransactionRepository
func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Create menyimpan data transaksi baru ke database
func (r *transactionRepository) Create(transaction *model.Transaction) error {
	return r.db.Create(transaction).Error
}

// FindByID mencari satu transaksi berdasarkan ID dan UserID (mencegah akses data user lain)
func (r *transactionRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Transaction, error) {
	var transaction model.Transaction

	err := r.db.
		Preload("Wallet").   // Muat relasi wallet
		Preload("Category"). // Muat relasi category
		Where("id = ? AND user_id = ?", id, userID).
		First(&transaction).Error

	if err != nil {
		return nil, err
	}

	return &transaction, nil
}

// FindAll mengambil semua transaksi milik user dengan dukungan filter
func (r *transactionRepository) FindAll(userID uuid.UUID, filter dto.TransactionFilterRequest) ([]model.Transaction, error) {
	var transactions []model.Transaction

	// Query dasar: hanya ambil transaksi milik user yang sedang login
	query := r.db.
		Preload("Wallet").
		Preload("Category").
		Where("user_id = ?", userID)

	// Filter opsional berdasarkan wallet
	if filter.WalletID != nil {
		query = query.Where("wallet_id = ?", *filter.WalletID)
	}

	// Filter opsional berdasarkan category
	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", *filter.CategoryID)
	}

	// Filter opsional berdasarkan tipe (income/expense)
	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}

	// Filter opsional berdasarkan rentang tanggal
	if filter.StartDate != nil {
		query = query.Where("transaction_date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("transaction_date <= ?", *filter.EndDate)
	}

	// Urutkan dari transaksi terbaru
	err := query.Order("transaction_date DESC").Find(&transactions).Error

	return transactions, err
}

// Update menyimpan perubahan data transaksi ke database
func (r *transactionRepository) Update(transaction *model.Transaction) error {
	return r.db.Save(transaction).Error
}

// Delete menghapus transaksi secara soft delete berdasarkan ID dan UserID
func (r *transactionRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	return r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Transaction{}).Error
}