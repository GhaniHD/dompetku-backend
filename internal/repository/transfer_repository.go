package repository

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransferRepository interface {
	Create(transfer *model.Transfer) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Transfer, error)
	FindAll(userID uuid.UUID, filter dto.TransferFilterRequest) ([]model.Transfer, error)
	Delete(id uuid.UUID, userID uuid.UUID) error
	IsFeeTransaction(transactionID uuid.UUID) (bool, error)
}

type transferRepository struct {
	db *gorm.DB
}

func NewTransferRepository(db *gorm.DB) TransferRepository {
	return &transferRepository{db: db}
}

func withTransferWallets(db *gorm.DB, userID uuid.UUID) *gorm.DB {
	scope := func(tx *gorm.DB) *gorm.DB {
		return tx.Unscoped().Where("user_id = ?", userID)
	}
	return db.Preload("FromWallet", scope).Preload("ToWallet", scope)
}

func (r *transferRepository) Create(transfer *model.Transfer) error {
	return r.db.Omit(clause.Associations).Create(transfer).Error
}

func (r *transferRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Transfer, error) {
	var transfer model.Transfer
	err := withTransferWallets(r.db, userID).
		Where("id = ? AND user_id = ?", id, userID).
		First(&transfer).Error
	if err != nil {
		return nil, err
	}
	return &transfer, nil
}

func (r *transferRepository) FindAll(userID uuid.UUID, filter dto.TransferFilterRequest) ([]model.Transfer, error) {
	var transfers []model.Transfer

	query := withTransferWallets(r.db, userID).Where("user_id = ?", userID)

	if filter.WalletID != "" {
		query = query.Where("from_wallet_id = ? OR to_wallet_id = ?", filter.WalletID, filter.WalletID)
	}
	if filter.StartDate != nil {
		query = query.Where("transfer_date >= CAST(? AS date)", utils.DateOnly(*filter.StartDate))
	}
	if filter.EndDate != nil {
		query = query.Where("transfer_date <= CAST(? AS date)", utils.DateOnly(*filter.EndDate))
	}

	err := query.Order("transfer_date DESC, created_at DESC").Find(&transfers).Error
	return transfers, err
}

func (r *transferRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	return r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Transfer{}).Error
}

func (r *transferRepository) IsFeeTransaction(transactionID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&model.Transfer{}).
		Where("fee_transaction_id = ?", transactionID).
		Count(&count).Error
	return count > 0, err
}
