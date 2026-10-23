package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	WalletTypeCash       = "cash"
	WalletTypeBank       = "bank"
	WalletTypeEWallet    = "ewallet"
	WalletTypeCreditCard = "credit_card"
	WalletTypeSavings    = "savings"
)

type Wallet struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID  uuid.UUID `gorm:"type:uuid;not null" json:"users_id"`
	Name    string    `gorm:"type:varchar(100)" json:"name"`
	Type    string    `gorm:"type:varchar(20);not null;default:'cash'" json:"type"`
	Balance float64   `gorm:"type:numeric(18,2);not null" json:"balance"`
	// Invarian: balance = opening_balance + Σ transaksi ± Σ transfer + Σ penyesuaian
	OpeningBalance float64        `gorm:"type:numeric(18,2);not null;default:0" json:"opening_balance"`
	IsArchived     bool           `gorm:"not null;default:false" json:"is_archived"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user"`
}

func (Wallet) TableName() string {
	return "wallets"
}

// AllowsNegative: hanya kartu kredit yang boleh bersaldo minus (tagihan berjalan).
func (w *Wallet) AllowsNegative() bool {
	return w.Type == WalletTypeCreditCard
}

// Usable berarti wallet boleh dipakai untuk transaksi/transfer baru.
func (w *Wallet) Usable() bool {
	return !w.DeletedAt.Valid && !w.IsArchived
}
