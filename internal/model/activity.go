package model

import (
	"time"

	"github.com/google/uuid"
)

// Jenis kejadian pada log aktivitas akun (disimpan 7 hari).
const (
	EventLogin           = "login"
	EventLoginFailed     = "login_failed"
	EventLoginLocked     = "login_locked"
	EventLogout          = "logout"
	EventPasswordChanged = "password_changed"
	EventPasswordReset   = "password_reset"
	EventForcedLogout    = "forced_logout"
	EventEmailVerified   = "email_verified"
	EventEmailChanged    = "email_changed"
)

type AuthEvent struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    *uuid.UUID `gorm:"type:uuid"`
	Email     string     `gorm:"type:varchar(100);not null;default:''"`
	Event     string     `gorm:"type:varchar(30);not null"`
	Detail    string     `gorm:"type:varchar(200);not null;default:''"`
	IP        string     `gorm:"type:varchar(64);not null;default:''"`
	Device    string     `gorm:"type:varchar(120);not null;default:''"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
}

func (AuthEvent) TableName() string { return "auth_events" }

const (
	AnnouncementInfo    = "info"
	AnnouncementWarning = "warning"
)

type Announcement struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Message   string    `gorm:"type:text;not null"`
	Level     string    `gorm:"type:varchar(10);not null;default:'info'"`
	EndsAt    *time.Time
	EndedAt   *time.Time
	CreatedBy uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (Announcement) TableName() string { return "announcements" }
