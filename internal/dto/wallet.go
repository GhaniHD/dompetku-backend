// internal/dto/wallet.go
package dto

import (
	"time"

	"github.com/google/uuid"
)

// ─── Request ──────────────────────────────────────────────────────────────────

type CreateWalletRequest struct {
	Name    string  `json:"name"    binding:"required,min=1,max=100"`
	Balance float64 `json:"balance" binding:"min=0"`
}

type UpdateWalletRequest struct {
	Name    *string  `json:"name"    binding:"omitempty,min=1,max=100"`
	Balance *float64 `json:"balance" binding:"omitempty,min=0"`
}

// ─── Response ─────────────────────────────────────────────────────────────────

type WalletResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Balance   float64   `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}