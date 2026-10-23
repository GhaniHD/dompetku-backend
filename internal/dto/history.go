package dto

import (
	"time"

	"github.com/google/uuid"
)

type WalletHistoryFilterRequest struct {
	StartDate *time.Time `form:"start_date"`
	EndDate   *time.Time `form:"end_date"`
	Page      int        `form:"page"  binding:"omitempty,min=1"`
	Limit     int        `form:"limit" binding:"omitempty,min=1,max=100"`
}

// WalletHistoryEntry: Amount bertanda (+ menambah saldo, − mengurangi saldo).
// Kind: income | expense | transfer_in | transfer_out | adjustment |
// debt_payable (hutang & cicilannya) | debt_receivable (piutang & pelunasannya).
type WalletHistoryEntry struct {
	ID           uuid.UUID `json:"id"`
	Source       string    `json:"source"`
	Kind         string    `json:"kind"`
	Amount       float64   `json:"amount"`
	Date         string    `json:"date"`
	CategoryName string    `json:"category_name,omitempty"`
	Counterpart  string    `json:"counterpart,omitempty"`
	Note         string    `json:"note"`
	BalanceAfter float64   `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

type WalletHistoryResponse struct {
	Wallet         WalletResponse       `json:"wallet"`
	OpeningBalance float64              `json:"opening_balance"`
	Entries        []WalletHistoryEntry `json:"entries"`
	Total          int64                `json:"total"`
	Page           int                  `json:"page"`
	Limit          int                  `json:"limit"`
}

type ExportFilterRequest struct {
	StartDate *time.Time `form:"start_date"`
	EndDate   *time.Time `form:"end_date"`
}
