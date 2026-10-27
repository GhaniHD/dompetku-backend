package repository

import (
	"dompetku/internal/model"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AchievedGoal struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Name         string
	TargetAmount float64
}

type GoalRepository interface {
	Create(goal *model.Goal) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Goal, error)
	FindAll(userID uuid.UUID) ([]model.Goal, error)
	ExistsForWallet(walletID uuid.UUID, excludeID *uuid.UUID) (bool, error)
	Update(goal *model.Goal) error
	Delete(id uuid.UUID, userID uuid.UUID) (int64, error)
	// MarkAchieved menandai target pada wallet yang saldonya kini >= target dan
	// belum pernah tercapai. Satu UPDATE sehingga notifikasi tidak terkirim ganda.
	MarkAchieved(walletID uuid.UUID, balance float64, at time.Time) ([]AchievedGoal, error)
	// ResetAchieved membatalkan status tercapai bila saldo turun lagi di bawah target.
	ResetAchieved(walletID uuid.UUID, balance float64) error
}

type goalRepository struct {
	db *gorm.DB
}

func NewGoalRepository(db *gorm.DB) GoalRepository {
	return &goalRepository{db: db}
}

func (r *goalRepository) withWallet(userID uuid.UUID) *gorm.DB {
	return r.db.Preload("Wallet", func(tx *gorm.DB) *gorm.DB {
		return tx.Unscoped().Where("user_id = ?", userID)
	})
}

func (r *goalRepository) Create(goal *model.Goal) error {
	return r.db.Omit(clause.Associations).Create(goal).Error
}

func (r *goalRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Goal, error) {
	var goal model.Goal
	if err := r.withWallet(userID).Where("id = ? AND user_id = ?", id, userID).First(&goal).Error; err != nil {
		return nil, err
	}
	return &goal, nil
}

func (r *goalRepository) FindAll(userID uuid.UUID) ([]model.Goal, error) {
	var goals []model.Goal
	err := r.withWallet(userID).
		Where("user_id = ?", userID).
		Order("achieved_at IS NOT NULL ASC, deadline ASC NULLS LAST, created_at DESC").
		Find(&goals).Error
	return goals, err
}

func (r *goalRepository) ExistsForWallet(walletID uuid.UUID, excludeID *uuid.UUID) (bool, error) {
	var count int64
	q := r.db.Model(&model.Goal{}).Where("wallet_id = ?", walletID)
	if excludeID != nil {
		q = q.Where("id <> ?", *excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *goalRepository) Update(goal *model.Goal) error {
	return r.db.Omit(clause.Associations).Save(goal).Error
}

func (r *goalRepository) Delete(id uuid.UUID, userID uuid.UUID) (int64, error) {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Goal{})
	return result.RowsAffected, result.Error
}

func (r *goalRepository) MarkAchieved(walletID uuid.UUID, balance float64, at time.Time) ([]AchievedGoal, error) {
	var rows []AchievedGoal
	err := r.db.Raw(`
		UPDATE goals SET achieved_at = ?
		WHERE wallet_id = ? AND deleted_at IS NULL AND achieved_at IS NULL AND target_amount <= ?
		RETURNING id, user_id, name, target_amount`, at, walletID, balance).Scan(&rows).Error
	return rows, err
}

func (r *goalRepository) ResetAchieved(walletID uuid.UUID, balance float64) error {
	return r.db.Exec(`UPDATE goals SET achieved_at = NULL
		WHERE wallet_id = ? AND deleted_at IS NULL AND achieved_at IS NOT NULL AND target_amount > ?`, walletID, balance).Error
}
