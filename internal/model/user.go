// internal/model/user.go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null"                      json:"name"`
	Email     string    `gorm:"type:varchar(100);uniqueIndex;not null"          json:"email"`
	Password  string    `gorm:"type:text;not null"                              json:"-"`
	AvatarURL string    `gorm:"type:varchar(255);default:''"                    json:"avatar_url"`
	// BudgetStartDay: tanggal mulai siklus budget (tanggal gajian), 1-31.
	BudgetStartDay int `gorm:"type:smallint;not null;default:1" json:"budget_start_day"`
	// IncomeType: salary (gaji tetap, periode ikut tanggal gajian) atau
	// irregular (tidak tetap/belum bekerja, periode = bulan kalender).
	IncomeType    string     `gorm:"type:varchar(12);not null;default:'salary'" json:"income_type"`
	IncomeSetupAt *time.Time `json:"-"`

	// Status akun: pending (menunggu admin), active, suspended, rejected.
	// Default kolom "active" agar akun lama tetap bisa login setelah migrasi;
	// pendaftaran baru diisi eksplisit oleh service.
	Role         string         `gorm:"type:varchar(10);not null;default:'user'" json:"role"`
	Status       string         `gorm:"type:varchar(12);not null;default:'active'" json:"status"`
	StatusReason string         `gorm:"type:text;not null;default:''" json:"-"`
	ApprovedAt   *time.Time     `json:"-"`
	ApprovedBy   *uuid.UUID     `gorm:"type:uuid" json:"-"`
	LastLoginAt  *time.Time     `json:"-"`
	TokenVersion int            `gorm:"not null;default:0" json:"-"`
	InviteID     *uuid.UUID     `gorm:"type:uuid" json:"-"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"                                  json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime"                                  json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index"                                           json:"-"`
}

const (
	IncomeSalary    = "salary"
	IncomeIrregular = "irregular"

	RoleUser  = "user"
	RoleAdmin = "admin"

	UserPending   = "pending"
	UserActive    = "active"
	UserSuspended = "suspended"
	UserRejected  = "rejected"
)

func (User) TableName() string {
	return "users"
}
