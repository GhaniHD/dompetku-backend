package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Transfer sengaja tidak didaftarkan ke AutoMigrate; tabelnya dibuat lewat
// migrations/002_create_transfers.up.sql.
type Transfer struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID           uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	FromWalletID     uuid.UUID      `gorm:"type:uuid;not null" json:"from_wallet_id"`
	ToWalletID       uuid.UUID      `gorm:"type:uuid;not null" json:"to_wallet_id"`
	Amount           float64        `gorm:"type:numeric(18,2);not null" json:"amount"`
	Fee              float64        `gorm:"type:numeric(18,2);not null;default:0" json:"fee"`
	FeeTransactionID *uuid.UUID     `gorm:"type:uuid" json:"fee_transaction_id"`
	Note             string         `gorm:"type:text;not null;default:''" json:"note"`
	TransferDate     time.Time      `gorm:"type:date;not null" json:"date"`
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	FromWallet Wallet `gorm:"foreignKey:FromWalletID" json:"-"`
	ToWallet   Wallet `gorm:"foreignKey:ToWalletID" json:"-"`
}

func (Transfer) TableName() string {
	return "transfers"
}
