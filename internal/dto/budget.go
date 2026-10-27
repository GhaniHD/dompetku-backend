package dto

import "github.com/google/uuid"

// ─── Request DTO ──────────────────────────────────────────────────────────────

// CreateBudgetRequest digunakan untuk membuat anggaran baru
// Kind "expense" (default) wajib category_id; "savings" wajib goal_id atau wallet_id.
type CreateBudgetRequest struct {
	Kind       string     `json:"kind"        binding:"omitempty,oneof=expense savings"`
	CategoryID *uuid.UUID `json:"category_id"`
	WalletID   *uuid.UUID `json:"wallet_id"`
	GoalID     *uuid.UUID `json:"goal_id"`
	Amount     float64    `json:"amount"      binding:"required,gt=0"`
	Month      int        `json:"month"       binding:"required,min=1,max=12"`
	Year       int        `json:"year"        binding:"required,min=2000"`
	Notes      string     `json:"notes"`
}

// UpdateBudgetRequest digunakan untuk mengubah anggaran (semua field opsional)
type UpdateBudgetRequest struct {
	Amount float64 `json:"amount" binding:"omitempty,gt=0"`
	Notes  string  `json:"notes"`
}

// BudgetFilterRequest digunakan sebagai query param saat mengambil list anggaran
type BudgetFilterRequest struct {
	Month int `form:"month" binding:"omitempty,min=1,max=12"`
	Year  int `form:"year"  binding:"omitempty,min=2000"`
}

// CopyBudgetRequest digunakan untuk menyalin anggaran dari bulan sebelumnya
type CopyBudgetRequest struct {
	FromMonth int `json:"from_month" binding:"required,min=1,max=12"`
	FromYear  int `json:"from_year"  binding:"required,min=2000"`
	ToMonth   int `json:"to_month"   binding:"required,min=1,max=12"`
	ToYear    int `json:"to_year"    binding:"required,min=2000"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

// BudgetResponse adalah response ringkas untuk list anggaran
type BudgetResponse struct {
	ID    uuid.UUID `json:"id"`
	Month int       `json:"month"`
	Year  int       `json:"year"`
	Notes string    `json:"notes"`

	// Rentang periode sesuai siklus user, format YYYY-MM-DD, keduanya inklusif.
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`

	Kind     string `json:"kind"`
	Category struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"category"`
	// Wallet dan Goal hanya terisi untuk anggaran tabungan.
	Wallet *BudgetRef `json:"wallet,omitempty"`
	Goal   *BudgetRef `json:"goal,omitempty"`

	// Untuk tabungan: Amount = rencana setoran, Spent = setoran bersih periode ini
	// (uang masuk − keluar dompet tabungan), Remain = sisa setor.
	Amount  float64 `json:"amount"`
	Spent   float64 `json:"spent"`
	Remain  float64 `json:"remain"`
	UsedPct float64 `json:"used_pct"`
	// expense: on_track | warning | used_up | over_budget; savings: in_progress | achieved
	Status string `json:"status"`
}

type BudgetRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// BudgetSummaryResponse adalah ringkasan semua anggaran dalam satu bulan/tahun
type BudgetSummaryResponse struct {
	Month       int              `json:"month"`
	Year        int              `json:"year"`
	PeriodStart string           `json:"period_start,omitempty"`
	PeriodEnd   string           `json:"period_end,omitempty"`
	StartDay    int              `json:"budget_start_day"`
	TotalBudget float64          `json:"total_budget"`
	TotalSpent  float64          `json:"total_spent"`
	TotalRemain float64          `json:"total_remain"`
	Budgets     []BudgetResponse `json:"budgets"`

	// Total di atas hanya anggaran pengeluaran; tabungan dijumlah terpisah.
	SavingsPlanned   float64 `json:"savings_planned"`
	SavingsDeposited float64 `json:"savings_deposited"`
	// FreeMoney hanya diisi untuk periode yang sedang berjalan.
	FreeMoney *FreeMoneyResponse `json:"free_money,omitempty"`
}

// FreeMoneyResponse: uang yang benar-benar bebas dipakai setelah menyisihkan
// sisa anggaran pengeluaran dan sisa setoran tabungan periode ini.
type FreeMoneyResponse struct {
	Spendable   float64 `json:"spendable"`
	BudgetLeft  float64 `json:"budget_left"`
	SavingsLeft float64 `json:"savings_left"`
	Amount      float64 `json:"amount"`
}
