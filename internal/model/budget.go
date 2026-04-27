package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Budget struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null" json:"users_id"`
	CategoryID uuid.UUID      `gorm:"type:uuid;not null" json:"category_id"`
	Amount     float64        `gorm:"type:decimal(10,2);not null" json:"amount"`
	Month      int            `gorm:"type:int;not null" json:"month"`
	Year       int            `gorm:"type:int;not null" json:"year"`
	Notes      *string        `gorm:"type:text" json:"notes"`         // ← tambahkan ini
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"` // ← tambahkan ini
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
 
	User     User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE;" json:"category"`
}

func (Budget) TableName() string {
	return "budgets"
}