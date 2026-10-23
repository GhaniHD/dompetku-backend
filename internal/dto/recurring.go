package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateRecurringRequest struct {
	WalletID   uuid.UUID  `json:"wallet_id"   binding:"required"`
	CategoryID uuid.UUID  `json:"category_id" binding:"required"`
	Type       string     `json:"type"        binding:"required,oneof=income expense"`
	Amount     float64    `json:"amount"      binding:"required,gt=0"`
	Note       string     `json:"note"        binding:"max=500"`
	Frequency  string     `json:"frequency"   binding:"required,oneof=weekly monthly"`
	StartDate  time.Time  `json:"start_date"  binding:"required"`
	EndDate    *time.Time `json:"end_date"`
}

// Frekuensi, tipe, dan tanggal mulai tidak bisa diubah; hapus lalu buat ulang.
type UpdateRecurringRequest struct {
	WalletID   *uuid.UUID `json:"wallet_id"`
	CategoryID *uuid.UUID `json:"category_id"`
	Amount     *float64   `json:"amount"    binding:"omitempty,gt=0"`
	Note       *string    `json:"note"      binding:"omitempty,max=500"`
	EndDate    *time.Time `json:"end_date"`
	IsActive   *bool      `json:"is_active"`
}

type RecurringResponse struct {
	ID          uuid.UUID      `json:"id"`
	Wallet      TransferWallet `json:"wallet"`
	Category    TransferWallet `json:"category"`
	Type        string         `json:"type"`
	Amount      float64        `json:"amount"`
	Note        string         `json:"note"`
	Frequency   string         `json:"frequency"`
	StartDate   string         `json:"start_date"`
	EndDate     *string        `json:"end_date"`
	NextRunDate string         `json:"next_run_date"`
	IsActive    bool           `json:"is_active"`
	LastRunAt   *time.Time     `json:"last_run_at"`
	LastError   string         `json:"last_error"`
}
