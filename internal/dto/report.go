package dto

import "github.com/google/uuid"

// ─── Request ──────────────────────────────────────────────────────────────────

type ReportFilterRequest struct {
	Month int `form:"month" binding:"required,min=1,max=12"`
	Year  int `form:"year"  binding:"required,min=2000"`
}

type YearlyReportFilterRequest struct {
	Year int `form:"year" binding:"required,min=2000"`
}

// ─── Response ─────────────────────────────────────────────────────────────────

type CategoryBreakdown struct {
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Total        float64   `json:"total"`
	Percentage   float64   `json:"percentage"`
	TxCount      int       `json:"transaction_count"`
}

type DailyTotal struct {
	Date    string  `json:"date"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
}

// MonthSummary adalah ringkasan per bulan dalam laporan tahunan
type MonthSummary struct {
	Month   int     `json:"month"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
}

type ReportResponse struct {
	Month int `json:"month"`
	Year  int `json:"year"`

	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	NetBalance   float64 `json:"net_balance"`

	IncomeByCategory  []CategoryBreakdown `json:"income_by_category"`
	ExpenseByCategory []CategoryBreakdown `json:"expense_by_category"`

	DailyTotals []DailyTotal `json:"daily_totals"`
}

// YearlyReportResponse adalah ringkasan 12 bulan dalam satu tahun
type YearlyReportResponse struct {
	Year int `json:"year"`

	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	NetBalance   float64 `json:"net_balance"`

	IncomeByCategory  []CategoryBreakdown `json:"income_by_category"`
	ExpenseByCategory []CategoryBreakdown `json:"expense_by_category"`

	MonthlyBreakdown []MonthSummary `json:"monthly_breakdown"`
}