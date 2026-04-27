package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Notification struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null" json:"users_id"`
	Title     string         `gorm:"type:varchar(100);not null" json:"title"`
	Message   string         `gorm:"type:varchar(255);not null" json:"message"`
	IsRead    bool           `gorm:"type:boolean;default:false" json:"read"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relasi ke User
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user"`
}

func (Notification) TableName() string {
	return "notifications"
}
