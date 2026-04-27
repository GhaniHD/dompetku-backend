// internal/service/analysis_service.go
package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/repository"
	claudepkg "dompetku/pkg/claude"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AnalysisService interface {
	GetAnalysis(userID uuid.UUID, year int) (*dto.AnalysisResponse, error)
}

type analysisService struct {
	analysisRepo repository.AnalysisRepository
	claudeClient *claudepkg.Client
}

func NewAnalysisService(
	analysisRepo repository.AnalysisRepository,
	claudeClient *claudepkg.Client,
) AnalysisService {
	return &analysisService{
		analysisRepo: analysisRepo,
		claudeClient: claudeClient,
	}
}

func (s *analysisService) GetAnalysis(userID uuid.UUID, year int) (*dto.AnalysisResponse, error) {
	// ── 1. Ambil semua data dari repository secara paralel ────────────
	trend, err := s.analysisRepo.GetMonthlyTrend(userID, year)
	if err != nil {
		return nil, err
	}

	expenseCat, err := s.analysisRepo.GetExpenseByCategory(userID, year)
	if err != nil {
		return nil, err
	}

	incomeCat, err := s.analysisRepo.GetIncomeByCategory(userID, year)
	if err != nil {
		return nil, err
	}

	// Ambil 6 bulan terakhir sebagai bahan prediksi & insight AI
	lastMonths, err := s.analysisRepo.GetLastNMonths(userID, 6)
	if err != nil {
		return nil, err
	}

	topCategories, err := s.analysisRepo.GetTopCategories(userID, 6)
	if err != nil {
		return nil, err
	}

	// ── 2. Hitung ringkasan angka ─────────────────────────────────────
	var totalIncome, totalExpense float64
	for _, t := range trend {
		totalIncome += t.Income
		totalExpense += t.Expense
	}
	netBalance := totalIncome - totalExpense
	savingsRate := 0.0
	if totalIncome > 0 {
		savingsRate = float64(int((netBalance/totalIncome)*10000)) / 100
	}

	// ── 3. Minta prediksi + insight ke Claude AI ──────────────────────
	insight, prediction, err := s.requestAIAnalysis(lastMonths, topCategories, year)
	if err != nil {
		// Jika AI gagal, tetap return data chart dengan insight kosong
		insight = dto.AIInsight{
			Summary:         "Analisis AI tidak tersedia saat ini.",
			Highlights:      []string{},
			Recommendations: []string{},
			RiskLevel:       "unknown",
			SpendingPattern: []dto.SpendingPattern{},
		}
		prediction = []dto.PredictionPoint{}
	}

	return &dto.AnalysisResponse{
		Year:              year,
		MonthlyTrend:      trend,
		ExpenseByCategory: expenseCat,
		IncomeByCategory:  incomeCat,
		Prediction:        prediction,
		TotalIncome:       totalIncome,
		TotalExpense:      totalExpense,
		NetBalance:        netBalance,
		SavingsRate:       savingsRate,
		Insight:           insight,
	}, nil
}

// requestAIAnalysis menyusun prompt dari data keuangan dan meminta analisis ke Claude.
func (s *analysisService) requestAIAnalysis(
	lastMonths []dto.MonthSummary,
	topCategories []dto.CategoryBreakdown,
	year int,
) (dto.AIInsight, []dto.PredictionPoint, error) {

	// ── Susun konteks data untuk prompt ──────────────────────────────
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Data keuangan pengguna tahun %d (6 bulan terakhir):\n\n", year))

	sb.WriteString("### Ringkasan Bulanan (Income vs Expense):\n")
	monthNames := []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
		"Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	for _, m := range lastMonths {
		label := fmt.Sprintf("Bulan %d", m.Month)
		if m.Month >= 1 && m.Month <= 12 {
			label = monthNames[m.Month]
		}
		sb.WriteString(fmt.Sprintf("- %s: Income=Rp%.0f, Expense=Rp%.0f, Net=Rp%.0f\n",
			label, m.Income, m.Expense, m.Net))
	}

	sb.WriteString("\n### Kategori Pengeluaran Terbesar (6 bulan terakhir):\n")
	for _, c := range topCategories {
		sb.WriteString(fmt.Sprintf("- %s: Rp%.0f (%.1f%% dari total, %d transaksi)\n",
			c.CategoryName, c.Total, c.Percentage, c.TxCount))
	}

	now := time.Now()
	nextMonths := make([]string, 3)
	for i := 0; i < 3; i++ {
		m := int(now.Month()) + i + 1
		y := year
		if m > 12 {
			m -= 12
			y++
		}
		nextMonths[i] = fmt.Sprintf("%s %d", monthNames[m], y)
	}

	// ── System prompt ─────────────────────────────────────────────────
	systemPrompt := `Kamu adalah analis keuangan personal yang membantu pengguna aplikasi manajemen keuangan.
Tugasmu adalah menganalisis data keuangan pengguna dan memberikan insight serta prediksi yang actionable.
Selalu gunakan bahasa Indonesia yang ramah dan mudah dipahami.
Berikan saran yang spesifik dan realistis berdasarkan data yang ada.
PENTING: Balas HANYA dengan JSON valid tanpa markdown, tanpa backtick, tanpa penjelasan tambahan di luar JSON.`

	// ── User prompt ───────────────────────────────────────────────────
	userPrompt := fmt.Sprintf(`%s

Berikan analisis dalam format JSON persis seperti ini (isi semua field, jangan tambah field baru):
{
  "summary": "ringkasan kondisi keuangan dalam 2-3 kalimat",
  "highlights": ["poin penting 1", "poin penting 2", "poin penting 3"],
  "recommendations": ["saran 1", "saran 2", "saran 3"],
  "risk_level": "aman",
  "spending_patterns": [
    {"category": "nama kategori", "avg_monthly": 0, "trend": "naik", "trend_pct": 0}
  ],
  "prediction": [
    {"month": %d, "label": "%s", "predicted_income": 0, "predicted_expense": 0, "predicted_net": 0, "confidence": "medium"},
    {"month": %d, "label": "%s", "predicted_income": 0, "predicted_expense": 0, "predicted_net": 0, "confidence": "medium"},
    {"month": %d, "label": "%s", "predicted_income": 0, "predicted_expense": 0, "predicted_net": 0, "confidence": "low"}
  ]
}

Aturan:
- risk_level hanya boleh: "aman", "perhatian", atau "kritis"
- trend hanya boleh: "naik", "turun", atau "stabil"  
- confidence hanya boleh: "high", "medium", atau "low"
- predicted_income dan predicted_expense harus angka positif
- Prediksi harus realistis berdasarkan tren data yang ada`,
		sb.String(),
		int(now.Month())+1, nextMonths[0],
		int(now.Month())+2, nextMonths[1],
		int(now.Month())+3, nextMonths[2],
	)

	// ── Kirim ke Claude ───────────────────────────────────────────────
	raw, err := s.claudeClient.Complete(systemPrompt, userPrompt)
	if err != nil {
		return dto.AIInsight{}, nil, err
	}

	// ── Parse JSON response ───────────────────────────────────────────
	// Bersihkan jika ada backtick yang lolos
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var aiResp struct {
		Summary         string               `json:"summary"`
		Highlights      []string             `json:"highlights"`
		Recommendations []string             `json:"recommendations"`
		RiskLevel       string               `json:"risk_level"`
		SpendingPattern []dto.SpendingPattern `json:"spending_patterns"`
		Prediction      []dto.PredictionPoint `json:"prediction"`
	}

	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return dto.AIInsight{}, nil, fmt.Errorf("gagal parse AI response: %w", err)
	}

	insight := dto.AIInsight{
		Summary:         aiResp.Summary,
		Highlights:      aiResp.Highlights,
		Recommendations: aiResp.Recommendations,
		RiskLevel:       aiResp.RiskLevel,
		SpendingPattern: aiResp.SpendingPattern,
	}

	return insight, aiResp.Prediction, nil
}