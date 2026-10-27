package dto

import (
	"time"

	"github.com/google/uuid"
)

type TransactionResponse struct {
	ID          uuid.UUID  `json:"id"`
	RecurringID *uuid.UUID `json:"recurring_id,omitempty"`
	Amount      float64    `json:"amount"`
	Type        string     `json:"type"`
	Note        string     `json:"note"`
	Date        time.Time  `json:"date"`

	WalletName   string    `json:"wallet_name"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name"`
}

type TransactionDetailResponse struct {
	ID          uuid.UUID  `json:"id"`
	RecurringID *uuid.UUID `json:"recurring_id,omitempty"`
	Amount      float64    `json:"amount"`
	Type        string     `json:"type"`
	Note        string     `json:"note"`
	Date        time.Time  `json:"date"`

	Wallet struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"wallet"`

	Category struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"category"`
}

type CreateTransactionRequest struct {
	WalletID   uuid.UUID `json:"wallet_id"   binding:"required"`
	CategoryID uuid.UUID `json:"category_id" binding:"required"`
	Amount     float64   `json:"amount"      binding:"required,gt=0"`
	Type       string    `json:"type"        binding:"required,oneof=income expense"`
	Note       string    `json:"note"`
	Date       time.Time `json:"date"        binding:"required"`
}

// Field pointer: nil = tidak diubah, sehingga note bisa dikosongkan dengan "".
type UpdateTransactionRequest struct {
	WalletID   *uuid.UUID `json:"wallet_id"`
	CategoryID *uuid.UUID `json:"category_id"`
	Amount     *float64   `json:"amount" binding:"omitempty,gt=0"`
	Type       *string    `json:"type"   binding:"omitempty,oneof=income expense"`
	Note       *string    `json:"note"`
	Date       *time.Time `json:"date"`
}

type TransactionFilterRequest struct {
	WalletID   string     `form:"wallet_id"   binding:"omitempty,uuid"`
	CategoryID string     `form:"category_id" binding:"omitempty,uuid"`
	Type       *string    `form:"type" binding:"omitempty,oneof=income expense"`
	StartDate  *time.Time `form:"start_date"`
	EndDate    *time.Time `form:"end_date"`
}
