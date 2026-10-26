package model

import (
	"time"

	"github.com/google/uuid"
)

type Invite struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code      string    `gorm:"type:varchar(32);not null;uniqueIndex"`
	Note      string    `gorm:"type:text;not null;default:''"`
	MaxUses   int       `gorm:"not null;default:1"`
	UsedCount int       `gorm:"not null;default:0"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	CreatedBy uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (Invite) TableName() string { return "invites" }

func (i *Invite) Usable(now time.Time) bool {
	return i.RevokedAt == nil && now.Before(i.ExpiresAt) && i.UsedCount < i.MaxUses
}

type PasswordResetToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	TokenHash string    `gorm:"type:varchar(64);not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedBy *uuid.UUID `gorm:"type:uuid"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
}

func (PasswordResetToken) TableName() string { return "password_reset_tokens" }

type AdminLog struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AdminID      uuid.UUID  `gorm:"type:uuid;not null"`
	Action       string     `gorm:"type:varchar(40);not null"`
	TargetUserID *uuid.UUID `gorm:"type:uuid"`
	Detail       string     `gorm:"type:text;not null;default:''"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
}

func (AdminLog) TableName() string { return "admin_logs" }
