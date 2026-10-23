package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BalanceAdjustment dibuat lewat migrations/003_wallet_types_adjustments.up.sql,
// tidak lewat AutoMigrate. Amount adalah selisih bertanda (new − previous).
type BalanceAdjustment struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID          uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	WalletID        uuid.UUID      `gorm:"type:uuid;not null" json:"wallet_id"`
	Amount          float64        `gorm:"type:numeric(18,2);not null" json:"amount"`
	PreviousBalance float64        `gorm:"type:numeric(18,2);not null" json:"previous_balance"`
	NewBalance      float64        `gorm:"type:numeric(18,2);not null" json:"new_balance"`
	Note            string         `gorm:"type:text;not null;default:''" json:"note"`
	AdjustedAt      time.Time      `gorm:"type:date;not null" json:"date"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (BalanceAdjustment) TableName() string {
	return "balance_adjustments"
}
