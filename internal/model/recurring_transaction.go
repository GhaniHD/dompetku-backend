package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	FrequencyWeekly  = "weekly"
	FrequencyMonthly = "monthly"
)

// RecurringTransaction dibuat lewat migrations/005_recurring_transactions.up.sql.
// AnchorDay = tanggal di start_date; untuk frekuensi bulanan tanggal 29–31 dipindah
// ke hari terakhir pada bulan yang lebih pendek tanpa menggeser bulan berikutnya.
type RecurringTransaction struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	WalletID    uuid.UUID      `gorm:"type:uuid;not null" json:"wallet_id"`
	CategoryID  uuid.UUID      `gorm:"type:uuid;not null" json:"category_id"`
	Type        string         `gorm:"type:varchar(10);not null" json:"type"`
	Amount      float64        `gorm:"type:numeric(18,2);not null" json:"amount"`
	Note        string         `gorm:"type:text;not null;default:''" json:"note"`
	Frequency   string         `gorm:"type:varchar(10);not null" json:"frequency"`
	AnchorDay   int            `gorm:"type:smallint;not null" json:"anchor_day"`
	StartDate   time.Time      `gorm:"type:date;not null" json:"start_date"`
	EndDate     *time.Time     `gorm:"type:date" json:"end_date"`
	NextRunDate time.Time      `gorm:"type:date;not null" json:"next_run_date"`
	IsActive    bool           `gorm:"not null;default:true" json:"is_active"`
	LastRunAt   *time.Time     `json:"last_run_at"`
	LastError   string         `gorm:"type:text;not null;default:''" json:"last_error"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Wallet   Wallet   `gorm:"foreignKey:WalletID" json:"-"`
	Category Category `gorm:"foreignKey:CategoryID" json:"-"`
}

func (RecurringTransaction) TableName() string {
	return "recurring_transactions"
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// NextAfter menghitung jadwal berikutnya setelah tanggal `current`.
func (r *RecurringTransaction) NextAfter(current time.Time) time.Time {
	current = dateOnly(current)
	if r.Frequency == FrequencyWeekly {
		return current.AddDate(0, 0, 7)
	}
	firstOfNext := time.Date(current.Year(), current.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstOfNext.AddDate(0, 1, -1).Day()
	day := r.AnchorDay
	if day > lastDay {
		day = lastDay
	}
	return time.Date(firstOfNext.Year(), firstOfNext.Month(), day, 0, 0, 0, 0, time.UTC)
}

func (r *RecurringTransaction) Finished(date time.Time) bool {
	return r.EndDate != nil && dateOnly(date).After(dateOnly(*r.EndDate))
}
