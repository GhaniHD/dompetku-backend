package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateWalletRequest struct {
	Name string `json:"name"    binding:"required,min=1,max=100"`
	Type string `json:"type"    binding:"omitempty,oneof=cash bank ewallet credit_card savings"`
	// Saldo awal boleh minus hanya untuk kartu kredit (dicek di service).
	Balance float64 `json:"balance"`
}

// Balance di sini dicatat sebagai penyesuaian saldo, bukan ditimpa langsung.
type UpdateWalletRequest struct {
	Name       *string  `json:"name"        binding:"omitempty,min=1,max=100"`
	Type       *string  `json:"type"        binding:"omitempty,oneof=cash bank ewallet credit_card savings"`
	Balance    *float64 `json:"balance"`
	IsArchived *bool    `json:"is_archived"`
}

type WalletFilterRequest struct {
	IncludeArchived bool `form:"include_archived"`
}

type AdjustBalanceRequest struct {
	ActualBalance *float64   `json:"actual_balance" binding:"required"`
	Note          string     `json:"note"           binding:"max=500"`
	Date          *time.Time `json:"date"`
}

type WalletResponse struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Balance    float64   `json:"balance"`
	IsArchived bool      `json:"is_archived"`
	CreatedAt  time.Time `json:"created_at"`
}

type DeleteWalletResponse struct {
	Archived bool `json:"archived"`
}

type BalanceAdjustmentResponse struct {
	ID              uuid.UUID `json:"id"`
	WalletID        uuid.UUID `json:"wallet_id"`
	Amount          float64   `json:"amount"`
	PreviousBalance float64   `json:"previous_balance"`
	NewBalance      float64   `json:"new_balance"`
	Note            string    `json:"note"`
	Date            time.Time `json:"date"`
	CreatedAt       time.Time `json:"created_at"`
}
