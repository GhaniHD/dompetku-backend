// internal/dto/analysis.go
package dto

// ─── Request ──────────────────────────────────────────────────────────────────

type AnalysisRequest struct {
	Year int `form:"year" binding:"required,min=2000"`
}

// ─── Chart Data ───────────────────────────────────────────────────────────────

// TrendPoint adalah satu titik data untuk chart tren bulanan
type TrendPoint struct {
	Month   int     `json:"month"`        // 1-12
	Label   string  `json:"label"`        // "Jan", "Feb", dst
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
}

// CategoryShare adalah porsi pengeluaran per kategori (untuk donut/pie chart)
type CategoryShare struct {
	CategoryName string  `json:"category_name"`
	Total        float64 `json:"total"`
	Percentage   float64 `json:"percentage"`
	Color        string  `json:"color"` // hex color untuk chart
}

// PredictionPoint adalah prediksi untuk bulan mendatang
type PredictionPoint struct {
	Month      int     `json:"month"`
	Label      string  `json:"label"`
	Income     float64 `json:"predicted_income"`
	Expense    float64 `json:"predicted_expense"`
	Net        float64 `json:"predicted_net"`
	Confidence string  `json:"confidence"` // "high" | "medium" | "low"
}

// ─── AI Insight ───────────────────────────────────────────────────────────────

type SpendingPattern struct {
	Category    string  `json:"category"`
	AvgMonthly  float64 `json:"avg_monthly"`
	Trend       string  `json:"trend"`  // "naik" | "turun" | "stabil"
	TrendPct    float64 `json:"trend_pct"`
}

type AIInsight struct {
	Summary         string            `json:"summary"`           // ringkasan kondisi keuangan
	Highlights      []string          `json:"highlights"`        // poin-poin penting
	Recommendations []string          `json:"recommendations"`   // saran actionable
	RiskLevel       string            `json:"risk_level"`        // "aman" | "perhatian" | "kritis"
	SpendingPattern []SpendingPattern `json:"spending_patterns"`
}

// ─── Full Response ────────────────────────────────────────────────────────────

type AnalysisResponse struct {
	Year int `json:"year"`

	// Data untuk chart
	MonthlyTrend      []TrendPoint    `json:"monthly_trend"`       // line chart 12 bulan
	ExpenseByCategory []CategoryShare `json:"expense_by_category"` // donut chart
	IncomeByCategory  []CategoryShare `json:"income_by_category"`  // donut chart
	Prediction        []PredictionPoint `json:"prediction"`        // prediksi 3 bulan ke depan

	// Ringkasan angka
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	NetBalance   float64 `json:"net_balance"`
	SavingsRate  float64 `json:"savings_rate"` // persen tabungan dari income

	// AI insight
	Insight AIInsight `json:"insight"`
}