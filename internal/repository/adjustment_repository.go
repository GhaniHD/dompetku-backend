package repository

import (
	"dompetku/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AdjustmentRepository interface {
	Create(adjustment *model.BalanceAdjustment) error
	FindByWallet(walletID uuid.UUID, userID uuid.UUID) ([]model.BalanceAdjustment, error)
}

type adjustmentRepository struct {
	db *gorm.DB
}

func NewAdjustmentRepository(db *gorm.DB) AdjustmentRepository {
	return &adjustmentRepository{db: db}
}

func (r *adjustmentRepository) Create(adjustment *model.BalanceAdjustment) error {
	return r.db.Create(adjustment).Error
}

func (r *adjustmentRepository) FindByWallet(walletID uuid.UUID, userID uuid.UUID) ([]model.BalanceAdjustment, error) {
	var adjustments []model.BalanceAdjustment
	err := r.db.
		Where("wallet_id = ? AND user_id = ?", walletID, userID).
		Order("adjusted_at DESC, created_at DESC").
		Find(&adjustments).Error
	return adjustments, err
}
