package dto

import (
    "time"
    "github.com/google/uuid"
)

// Response DTO (List / Detail)
type TransactionResponse struct {
	ID        uuid.UUID `json:"id"`
	Amount    float64   `json:"amount"`
	Type      string    `json:"type"`
	Note      string    `json:"note"`
	Date      time.Time `json:"date"`

	WalletName   string `json:"wallet_name"`
	CategoryName string `json:"category_name"`
}

// Detail Transaction DTO
type TransactionDetailResponse struct {
	ID        uuid.UUID `json:"id"`
	Amount    float64   `json:"amount"`
	Type      string    `json:"type"`
	Note      string    `json:"note"`
	Date      time.Time `json:"date"`

	Wallet struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"wallet"`

	Category struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"category"`
}

// Create Transaction DTO
type CreateTransactionRequest struct {
	WalletID   uuid.UUID `json:"wallet_id" binding:"required"`
	CategoryID uuid.UUID `json:"category_id" binding:"required"`
	Amount     float64   `json:"amount" binding:"required"`
	Type       string    `json:"type" binding:"required"` // income / expense
	Note       string    `json:"note"`
	Date       time.Time `json:"date" binding:"required"`
}

// Update Transaction DTO
type UpdateTransactionRequest struct {
	WalletID   uuid.UUID `json:"wallet_id"`
	CategoryID uuid.UUID `json:"category_id"`
	Amount     float64   `json:"amount"`
	Type       string    `json:"type"`
	Note       string    `json:"note"`
	Date       time.Time `json:"date"`
}

// Filter / Query DTO
type TransactionFilterRequest struct {
	WalletID   *uuid.UUID `form:"wallet_id"`
	CategoryID *uuid.UUID `form:"category_id"`
	Type       *string    `form:"type"`
	StartDate  *time.Time `form:"start_date"`
	EndDate    *time.Time `form:"end_date"`
}