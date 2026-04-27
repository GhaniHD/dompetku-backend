package model

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Transaction struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null" json:"users_id"`
	WalletID    uuid.UUID      `gorm:"type:uuid;not null" json:"wallet_id"`
	CategoryID  uuid.UUID      `gorm:"type:uuid;not null" json:"category_id"`
	Amount      float64        `gorm:"type:decimal(10,2);not null" json:"amount"`
	Type 	  string         `gorm:"type:varchar(10);check:type IN ('income','expense')" json:"type"`
	Note string         `gorm:"type:text" json:"note"`
	TransactionDate        time.Time      `gorm:"type:date;not null" json:"date"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relasi ke User
	User        User           `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user"`
	// Relasi ke Wallet
	Wallet      Wallet         `gorm:"foreignKey:WalletID;constraint:OnDelete:CASCADE;" json:"wallet"`
	// Relasi ke Category
	Category    Category       `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE;" json:"category"`
}

func (Transaction) TableName() string {
	return "transactions"
}