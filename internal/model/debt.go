package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DebtPayable    = "payable"
	DebtReceivable = "receivable"

	DebtMovementPrincipal = "principal"
	DebtMovementPayment   = "payment"
)

// Debt dan DebtMovement dibuat lewat migrations/006_debts_goals.up.sql.
// payable = user berhutang ke orang lain; receivable = orang lain berhutang ke user.
type Debt struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null"`
	Kind           string         `gorm:"type:varchar(10);not null"`
	Counterparty   string         `gorm:"type:varchar(100);not null"`
	Principal      float64        `gorm:"type:numeric(18,2);not null"`
	PaidAmount     float64        `gorm:"type:numeric(18,2);not null;default:0"`
	Note           string         `gorm:"type:text;not null;default:''"`
	DueDate        *time.Time     `gorm:"type:date"`
	LastRemindedOn *time.Time     `gorm:"type:date"`
	CreatedAt      time.Time      `gorm:"autoCreateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	Movements []DebtMovement `gorm:"foreignKey:DebtID"`
}

func (Debt) TableName() string {
	return "debts"
}

func (d *Debt) Remaining() float64 {
	return d.Principal - d.PaidAmount
}

// WalletSign: arah perubahan saldo saat pokok hutang dicatat. Pembayaran memakai
// tanda kebalikannya.
func (d *Debt) WalletSign() float64 {
	if d.Kind == DebtPayable {
		return 1
	}
	return -1
}

type DebtMovement struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DebtID       uuid.UUID      `gorm:"type:uuid;not null"`
	UserID       uuid.UUID      `gorm:"type:uuid;not null"`
	WalletID     *uuid.UUID     `gorm:"type:uuid"`
	Kind         string         `gorm:"type:varchar(10);not null"`
	Amount       float64        `gorm:"type:numeric(18,2);not null"`
	WalletDelta  float64        `gorm:"type:numeric(18,2);not null;default:0"`
	MovementDate time.Time      `gorm:"type:date;not null"`
	Note         string         `gorm:"type:text;not null;default:''"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	Wallet *Wallet `gorm:"foreignKey:WalletID"`
}

func (DebtMovement) TableName() string {
	return "debt_movements"
}
