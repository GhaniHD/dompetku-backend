package repository

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DebtReminder struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Kind         string
	Counterparty string
	Principal    float64
	PaidAmount   float64
	DueDate      time.Time
}

type DebtRepository interface {
	Create(debt *model.Debt) error
	FindByID(id uuid.UUID, userID uuid.UUID) (*model.Debt, error)
	FindByIDForUpdate(id uuid.UUID, userID uuid.UUID) (*model.Debt, error)
	FindAll(userID uuid.UUID, filter dto.DebtFilterRequest) ([]model.Debt, error)
	Update(debt *model.Debt) error
	Delete(id uuid.UUID) error
	CreateMovement(m *model.DebtMovement) error
	FindMovement(id uuid.UUID, debtID uuid.UUID) (*model.DebtMovement, error)
	DeleteMovement(id uuid.UUID) error
	// ClaimReminders menandai hutang yang jatuh tempo hari ini atau besok sebagai
	// sudah diingatkan, lalu mengembalikannya. Satu UPDATE sehingga aman dipanggil
	// paralel dan tidak mengirim pengingat ganda.
	ClaimReminders(today time.Time) ([]DebtReminder, error)
}

type debtRepository struct {
	db *gorm.DB
}

func NewDebtRepository(db *gorm.DB) DebtRepository {
	return &debtRepository{db: db}
}

func (r *debtRepository) Create(debt *model.Debt) error {
	return r.db.Omit(clause.Associations).Create(debt).Error
}

func (r *debtRepository) withMovements(userID uuid.UUID) *gorm.DB {
	return r.db.Preload("Movements", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("movement_date ASC, created_at ASC")
	}).Preload("Movements.Wallet", func(tx *gorm.DB) *gorm.DB {
		return tx.Unscoped().Where("user_id = ?", userID)
	})
}

func (r *debtRepository) FindByID(id uuid.UUID, userID uuid.UUID) (*model.Debt, error) {
	var debt model.Debt
	err := r.withMovements(userID).Where("id = ? AND user_id = ?", id, userID).First(&debt).Error
	if err != nil {
		return nil, err
	}
	return &debt, nil
}

func (r *debtRepository) FindByIDForUpdate(id uuid.UUID, userID uuid.UUID) (*model.Debt, error) {
	var debt model.Debt
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", id, userID).
		First(&debt).Error
	if err != nil {
		return nil, err
	}
	return &debt, nil
}

func (r *debtRepository) FindAll(userID uuid.UUID, filter dto.DebtFilterRequest) ([]model.Debt, error) {
	var debts []model.Debt
	q := r.db.Where("user_id = ?", userID)
	if filter.Kind != "" {
		q = q.Where("kind = ?", filter.Kind)
	}
	switch filter.Status {
	case "open":
		q = q.Where("paid_amount < principal")
	case "paid":
		q = q.Where("paid_amount >= principal")
	}
	err := q.Order("(paid_amount >= principal) ASC, due_date ASC NULLS LAST, created_at DESC").Find(&debts).Error
	return debts, err
}

func (r *debtRepository) Update(debt *model.Debt) error {
	return r.db.Omit(clause.Associations).Save(debt).Error
}

func (r *debtRepository) Delete(id uuid.UUID) error {
	if err := r.db.Where("debt_id = ?", id).Delete(&model.DebtMovement{}).Error; err != nil {
		return err
	}
	return r.db.Where("id = ?", id).Delete(&model.Debt{}).Error
}

func (r *debtRepository) CreateMovement(m *model.DebtMovement) error {
	return r.db.Omit(clause.Associations).Create(m).Error
}

func (r *debtRepository) FindMovement(id uuid.UUID, debtID uuid.UUID) (*model.DebtMovement, error) {
	var m model.DebtMovement
	if err := r.db.Where("id = ? AND debt_id = ?", id, debtID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *debtRepository) DeleteMovement(id uuid.UUID) error {
	return r.db.Where("id = ?", id).Delete(&model.DebtMovement{}).Error
}

func (r *debtRepository) ClaimReminders(today time.Time) ([]DebtReminder, error) {
	var rows []DebtReminder
	day := today.Format("2006-01-02")
	err := r.db.Raw(`
		UPDATE debts SET last_reminded_on = CAST(@today AS date)
		WHERE deleted_at IS NULL
		  AND paid_amount < principal
		  AND due_date BETWEEN CAST(@today AS date) AND CAST(@today AS date) + 1
		  AND (last_reminded_on IS NULL OR last_reminded_on < CAST(@today AS date))
		RETURNING id, user_id, kind, counterparty, principal, paid_amount, due_date`,
		map[string]any{"today": day}).Scan(&rows).Error
	return rows, err
}
