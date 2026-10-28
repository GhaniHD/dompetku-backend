package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	CodeVerify = "verify"
	CodeChange = "change"
)

// EmailCode: kode OTP 6 digit; yang disimpan hanya hash-nya.
type EmailCode struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	Email     string    `gorm:"type:varchar(100);not null"`
	Purpose   string    `gorm:"type:varchar(10);not null"`
	CodeHash  string    `gorm:"type:varchar(64);not null"`
	Attempts  int       `gorm:"not null;default:0"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (EmailCode) TableName() string { return "email_codes" }

const (
	MailPending = "pending"
	MailSent    = "sent"
	MailFailed  = "failed"
	MailSkipped = "skipped"
)

// Jenis email. Prioritas 0 = penting (kode, reset password), selalu dikirim duluan.
const (
	MailKindCode         = "code"
	MailKindReset        = "reset"
	MailKindApproved     = "approved"
	MailKindEmailChanged = "email_changed"
	MailKindLoginAlert   = "login_alert"
	MailKindReminder     = "reminder"
	MailKindWeekly       = "weekly"
	MailKindNewSignup    = "new_signup"
)

type EmailOutbox struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID        *uuid.UUID `gorm:"type:uuid"`
	ToEmail       string     `gorm:"type:varchar(100);not null"`
	Subject       string     `gorm:"type:varchar(200);not null"`
	HTML          string     `gorm:"column:html;type:text;not null"`
	Text          string     `gorm:"type:text;not null"`
	Kind          string     `gorm:"type:varchar(20);not null"`
	Priority      int        `gorm:"not null"`
	Unsubscribe   string     `gorm:"type:text;not null;default:''"`
	Status        string     `gorm:"type:varchar(10);not null;default:'pending'"`
	Attempts      int        `gorm:"not null;default:0"`
	NextAttemptAt time.Time  `gorm:"not null"`
	LastError     string     `gorm:"type:varchar(300);not null;default:''"`
	SentAt        *time.Time
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

func (EmailOutbox) TableName() string { return "email_outbox" }
