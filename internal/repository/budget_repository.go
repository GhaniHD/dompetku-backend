package repository

import (
	"dompetku/internal/model"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BudgetRepository mendefinisikan kontrak operasi database untuk anggaran
type BudgetRepository interface {
	Create(budget *model.Budget) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Budget, error)
	FindAll(userID uuid.UUID, month, year int) ([]model.Budget, error)
	FindByCategory(userID uuid.UUID, categoryID uuid.UUID, month, year int) (*model.Budget, error)
	FindSavingsByWallet(userID, walletID uuid.UUID, month, year int) (*model.Budget, error)
	Update(budget *model.Budget) error
	Delete(id uuid.UUID, userID uuid.UUID) error

	// GetSpentByCategory menjumlahkan pengeluaran kategori dalam rentang [start, end),
	// yaitu periode budget sesuai tanggal mulai siklus user.
	GetSpentByCategory(userID uuid.UUID, categoryID uuid.UUID, start, end time.Time) (float64, error)
}

type budgetRepository struct {
	db *gorm.DB
}

func NewBudgetRepository(db *gorm.DB) BudgetRepository {
	return &budgetRepository{db: db}
}

func (r *budgetRepository) Create(budget *model.Budget) error {
	return r.db.Create(budget).Error
}

func (r *budgetRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Budget, error) {
	var budget model.Budget
	err := r.db.
		Preload("Category").
		Where("id = ? AND user_id = ?", id, userID).
		First(&budget).Error
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *budgetRepository) FindAll(userID uuid.UUID, month, year int) ([]model.Budget, error) {
	var budgets []model.Budget

	query := r.db.Preload("Category").Where("user_id = ?", userID)

	if month > 0 {
		query = query.Where("month = ?", month)
	}
	if year > 0 {
		query = query.Where("year = ?", year)
	}

	err := query.Order("year DESC, month DESC").Find(&budgets).Error
	return budgets, err
}

func (r *budgetRepository) FindByCategory(userID uuid.UUID, categoryID uuid.UUID, month, year int) (*model.Budget, error) {
	var budget model.Budget
	err := r.db.
		Where("user_id = ? AND category_id = ? AND month = ? AND year = ?",
			userID, categoryID, month, year).
		First(&budget).Error
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *budgetRepository) FindSavingsByWallet(userID, walletID uuid.UUID, month, year int) (*model.Budget, error) {
	var budget model.Budget
	err := r.db.
		Where("user_id = ? AND kind = ? AND wallet_id = ? AND month = ? AND year = ?",
			userID, model.BudgetSavings, walletID, month, year).
		First(&budget).Error
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *budgetRepository) Update(budget *model.Budget) error {
	return r.db.Save(budget).Error
}

func (r *budgetRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	return r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Budget{}).Error
}

func (r *budgetRepository) GetSpentByCategory(userID uuid.UUID, categoryID uuid.UUID, start, end time.Time) (float64, error) {
	var total float64

	err := r.db.
		Table("transactions").
		Select("COALESCE(SUM(amount), 0)").
		Where(`user_id = ?
			AND category_id = ?
			AND type = 'expense'
			AND transaction_date >= ?
			AND transaction_date < ?
			AND deleted_at IS NULL`,
			userID, categoryID, start.Format("2006-01-02"), end.Format("2006-01-02")).
		Scan(&total).Error

	return total, err
}
