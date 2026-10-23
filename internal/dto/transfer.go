package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateTransferRequest struct {
	FromWalletID uuid.UUID `json:"from_wallet_id" binding:"required"`
	ToWalletID   uuid.UUID `json:"to_wallet_id"   binding:"required"`
	Amount       float64   `json:"amount"         binding:"required,gt=0"`
	Fee          float64   `json:"fee"            binding:"gte=0"`
	Note         string    `json:"note"           binding:"max=500"`
	Date         time.Time `json:"date"           binding:"required"`
}

type TransferFilterRequest struct {
	WalletID  string     `form:"wallet_id" binding:"omitempty,uuid"`
	StartDate *time.Time `form:"start_date"`
	EndDate   *time.Time `form:"end_date"`
}

type TransferWallet struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type TransferResponse struct {
	ID               uuid.UUID      `json:"id"`
	FromWallet       TransferWallet `json:"from_wallet"`
	ToWallet         TransferWallet `json:"to_wallet"`
	Amount           float64        `json:"amount"`
	Fee              float64        `json:"fee"`
	FeeTransactionID *uuid.UUID     `json:"fee_transaction_id"`
	Note             string         `json:"note"`
	Date             time.Time      `json:"date"`
	CreatedAt        time.Time      `json:"created_at"`
}
