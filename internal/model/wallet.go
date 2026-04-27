package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Wallet struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null" json:"users_id"`
	Name      string         `gorm:"type:varchar(100)" json:"name"`
	Balance   float64        `gorm:"type:decimal(10,2);not null" json:"balance"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relasi ke User
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user"`
}

func (Wallet) TableName() string {
	return "wallets"
}
