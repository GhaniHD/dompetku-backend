package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	BudgetExpense = "expense"
	BudgetSavings = "savings"
)

// Budget berjenis expense membatasi pengeluaran satu kategori; berjenis savings
// adalah rencana setoran ke satu dompet tabungan (opsional terhubung ke target).
type Budget struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null" json:"users_id"`
	Kind       string         `gorm:"type:varchar(10);not null;default:'expense'" json:"kind"`
	CategoryID *uuid.UUID     `gorm:"type:uuid" json:"category_id"`
	WalletID   *uuid.UUID     `gorm:"type:uuid" json:"wallet_id"`
	GoalID     *uuid.UUID     `gorm:"type:uuid" json:"goal_id"`
	Amount     float64        `gorm:"type:numeric(18,2);not null" json:"amount"`
	Month      int            `gorm:"type:int;not null" json:"month"`
	Year       int            `gorm:"type:int;not null" json:"year"`
	Notes      *string        `gorm:"type:text" json:"notes"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	User     User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE;" json:"category"`
}

func (Budget) TableName() string {
	return "budgets"
}
