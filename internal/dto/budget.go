package dto

import "github.com/google/uuid"

// ─── Request DTO ──────────────────────────────────────────────────────────────

// CreateBudgetRequest digunakan untuk membuat anggaran baru
type CreateBudgetRequest struct {
	CategoryID uuid.UUID `json:"category_id" binding:"required"`
	Amount     float64   `json:"amount"      binding:"required,gt=0"`
	Month      int       `json:"month"       binding:"required,min=1,max=12"`
	Year       int       `json:"year"        binding:"required,min=2000"`
	Notes      string    `json:"notes"`
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

	Category struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"category"`

	Amount  float64 `json:"amount"`  // anggaran yang ditetapkan
	Spent   float64 `json:"spent"`   // total pengeluaran aktual dari transaksi
	Remain  float64 `json:"remain"`  // sisa anggaran (amount - spent)
	UsedPct float64 `json:"used_pct"` // persentase penggunaan (0–100+)
	Status  string  `json:"status"`  // "on_track" | "warning" | "over_budget"
}

// BudgetSummaryResponse adalah ringkasan semua anggaran dalam satu bulan/tahun
type BudgetSummaryResponse struct {
	Month       int              `json:"month"`
	Year        int              `json:"year"`
	TotalBudget float64          `json:"total_budget"`
	TotalSpent  float64          `json:"total_spent"`
	TotalRemain float64          `json:"total_remain"`
	Budgets     []BudgetResponse `json:"budgets"`
}