package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Category struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	
	UserID    uuid.UUID      `gorm:"type:uuid;not null" json:"users_id"`
	
	Name      string         `gorm:"type:varchar(100)" json:"name"`
	Type      string         `gorm:"type:varchar(10);check:type IN ('income','expense')" json:"type"`
	
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	
	// Relasi ke User
	User      User           `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user"`
}

func (Category) TableName() string {
	return "categories"
}