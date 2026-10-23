package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Goal dibuat lewat migrations/006_debts_goals.up.sql. Progres dihitung dari saldo
// wallet yang terhubung, satu wallet hanya untuk satu target aktif.
type Goal struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID       uuid.UUID      `gorm:"type:uuid;not null"`
	WalletID     uuid.UUID      `gorm:"type:uuid;not null"`
	Name         string         `gorm:"type:varchar(100);not null"`
	TargetAmount float64        `gorm:"type:numeric(18,2);not null"`
	Deadline     *time.Time     `gorm:"type:date"`
	AchievedAt   *time.Time     `gorm:""`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	Wallet Wallet `gorm:"foreignKey:WalletID"`
}

func (Goal) TableName() string {
	return "goals"
}
