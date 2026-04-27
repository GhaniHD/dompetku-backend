// internal/model/user.go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name      string         `gorm:"type:varchar(100);not null"                      json:"name"`
	Email     string         `gorm:"type:varchar(100);uniqueIndex;not null"          json:"email"`
	Password  string         `gorm:"type:text;not null"                              json:"-"`
	AvatarURL string         `gorm:"type:varchar(255);default:''"                    json:"avatar_url"`
	CreatedAt time.Time      `gorm:"autoCreateTime"                                  json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"                                  json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                           json:"-"`
}

func (User) TableName() string {
	return "users"
}