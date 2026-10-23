package repository

import (
	"dompetku/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RecurringRepository interface {
	Create(rule *model.RecurringTransaction) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.RecurringTransaction, error)
	FindAll(userID uuid.UUID) ([]model.RecurringTransaction, error)
	Update(rule *model.RecurringTransaction) error
	Delete(id uuid.UUID, userID uuid.UUID) (int64, error)
}

type recurringRepository struct {
	db *gorm.DB
}

func NewRecurringRepository(db *gorm.DB) RecurringRepository {
	return &recurringRepository{db: db}
}

func withRecurringRelations(db *gorm.DB, userID uuid.UUID) *gorm.DB {
	scope := func(tx *gorm.DB) *gorm.DB {
		return tx.Unscoped().Where("user_id = ?", userID)
	}
	return db.Preload("Wallet", scope).Preload("Category", scope)
}

func (r *recurringRepository) Create(rule *model.RecurringTransaction) error {
	return r.db.Omit(clause.Associations).Create(rule).Error
}

func (r *recurringRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.RecurringTransaction, error) {
	var rule model.RecurringTransaction
	err := withRecurringRelations(r.db, userID).
		Where("id = ? AND user_id = ?", id, userID).
		First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *recurringRepository) FindAll(userID uuid.UUID) ([]model.RecurringTransaction, error) {
	var rules []model.RecurringTransaction
	err := withRecurringRelations(r.db, userID).
		Where("user_id = ?", userID).
		Order("is_active DESC, next_run_date ASC").
		Find(&rules).Error
	return rules, err
}

func (r *recurringRepository) Update(rule *model.RecurringTransaction) error {
	return r.db.Omit(clause.Associations).Save(rule).Error
}

func (r *recurringRepository) Delete(id uuid.UUID, userID uuid.UUID) (int64, error) {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.RecurringTransaction{})
	return result.RowsAffected, result.Error
}
