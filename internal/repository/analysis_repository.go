// internal/repository/analysis_repository.go
package repository

import (
	"dompetku/internal/dto"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AnalysisRepository interface {
	// GetMonthlyTrend mengambil income & expense per bulan selama N bulan terakhir
	GetMonthlyTrend(userID uuid.UUID, year int) ([]dto.TrendPoint, error)
	// GetExpenseByCategory mengambil breakdown pengeluaran per kategori dalam setahun
	GetExpenseByCategory(userID uuid.UUID, year int) ([]dto.CategoryShare, error)
	// GetIncomeByCategory mengambil breakdown pemasukan per kategori dalam setahun
	GetIncomeByCategory(userID uuid.UUID, year int) ([]dto.CategoryShare, error)
	// GetLastNMonths mengambil summary N bulan terakhir (untuk bahan prediksi AI)
	GetLastNMonths(userID uuid.UUID, n int) ([]dto.MonthSummary, error)
	// GetTopCategories mengambil kategori pengeluaran terbesar N bulan terakhir
	GetTopCategories(userID uuid.UUID, n int) ([]dto.CategoryBreakdown, error)
}

type analysisRepository struct {
	db *gorm.DB
}

func NewAnalysisRepository(db *gorm.DB) AnalysisRepository {
	return &analysisRepository{db: db}
}

func (r *analysisRepository) GetMonthlyTrend(userID uuid.UUID, year int) ([]dto.TrendPoint, error) {
	type rawRow struct {
		Month   int     `gorm:"column:month"`
		Income  float64 `gorm:"column:income"`
		Expense float64 `gorm:"column:expense"`
	}
	var rows []rawRow
	err := r.db.Table("transactions").
		Select(`EXTRACT(MONTH FROM transaction_date)::int AS month,
			COALESCE(SUM(CASE WHEN type='income'  THEN amount ELSE 0 END),0) AS income,
			COALESCE(SUM(CASE WHEN type='expense' THEN amount ELSE 0 END),0) AS expense`).
		Where(`user_id = ? AND EXTRACT(YEAR FROM transaction_date) = ? AND deleted_at IS NULL`,
			userID, year).
		Group("month").Order("month ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	monthLabels := []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
		"Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

	result := make([]dto.TrendPoint, 0, len(rows))
	for _, row := range rows {
		label := ""
		if row.Month >= 1 && row.Month <= 12 {
			label = monthLabels[row.Month]
		}
		result = append(result, dto.TrendPoint{
			Month:   row.Month,
			Label:   label,
			Income:  row.Income,
			Expense: row.Expense,
			Net:     row.Income - row.Expense,
		})
	}
	return result, nil
}

func (r *analysisRepository) GetExpenseByCategory(userID uuid.UUID, year int) ([]dto.CategoryShare, error) {
	return r.getCategoryShare(userID, "expense", year)
}

func (r *analysisRepository) GetIncomeByCategory(userID uuid.UUID, year int) ([]dto.CategoryShare, error) {
	return r.getCategoryShare(userID, "income", year)
}

func (r *analysisRepository) getCategoryShare(userID uuid.UUID, txType string, year int) ([]dto.CategoryShare, error) {
	type rawRow struct {
		CategoryName string  `gorm:"column:category_name"`
		Total        float64 `gorm:"column:total"`
	}
	var rows []rawRow
	err := r.db.Table("transactions t").
		Select("c.name AS category_name, COALESCE(SUM(t.amount),0) AS total").
		Joins("JOIN categories c ON c.id = t.category_id").
		Where(`t.user_id = ? AND t.type = ? AND EXTRACT(YEAR FROM t.transaction_date) = ? AND t.deleted_at IS NULL`,
			userID, txType, year).
		Group("c.name").Order("total DESC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Hitung total keseluruhan untuk persentase
	var grand float64
	for _, row := range rows {
		grand += row.Total
	}

	// Palet warna untuk chart
	colors := []string{
		"#6366f1", "#8b5cf6", "#ec4899", "#f59e0b",
		"#10b981", "#3b82f6", "#ef4444", "#14b8a6",
		"#f97316", "#84cc16",
	}

	result := make([]dto.CategoryShare, 0, len(rows))
	for i, row := range rows {
		pct := 0.0
		if grand > 0 {
			pct = float64(int((row.Total/grand)*10000)) / 100
		}
		color := colors[i%len(colors)]
		result = append(result, dto.CategoryShare{
			CategoryName: row.CategoryName,
			Total:        row.Total,
			Percentage:   pct,
			Color:        color,
		})
	}
	return result, nil
}

func (r *analysisRepository) GetLastNMonths(userID uuid.UUID, n int) ([]dto.MonthSummary, error) {
	type rawRow struct {
		Month   int     `gorm:"column:month"`
		Year    int     `gorm:"column:year"`
		Income  float64 `gorm:"column:income"`
		Expense float64 `gorm:"column:expense"`
	}
	var rows []rawRow
	err := r.db.Table("transactions").
		Select(`EXTRACT(MONTH FROM transaction_date)::int AS month,
			EXTRACT(YEAR  FROM transaction_date)::int AS year,
			COALESCE(SUM(CASE WHEN type='income'  THEN amount ELSE 0 END),0) AS income,
			COALESCE(SUM(CASE WHEN type='expense' THEN amount ELSE 0 END),0) AS expense`).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Group("year, month").
		Order("year DESC, month DESC").
		Limit(n).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Balik urutan agar dari yang terlama ke terbaru
	result := make([]dto.MonthSummary, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		result = append(result, dto.MonthSummary{
			Month:   row.Month,
			Income:  row.Income,
			Expense: row.Expense,
			Net:     row.Income - row.Expense,
		})
	}
	return result, nil
}

func (r *analysisRepository) GetTopCategories(userID uuid.UUID, n int) ([]dto.CategoryBreakdown, error) {
	type rawRow struct {
		CategoryName string  `gorm:"column:category_name"`
		Total        float64 `gorm:"column:total"`
		TxCount      int     `gorm:"column:tx_count"`
	}
	var rows []rawRow

	// Ambil data N bulan terakhir
	err := r.db.Table("transactions t").
		Select("c.name AS category_name, COALESCE(SUM(t.amount),0) AS total, COUNT(t.id) AS tx_count").
		Joins("JOIN categories c ON c.id = t.category_id").
		Where(`t.user_id = ? AND t.type = 'expense' AND t.deleted_at IS NULL
			AND t.transaction_date >= NOW() - INTERVAL '6 months'`, userID).
		Group("c.name").Order("total DESC").Limit(5).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var grand float64
	for _, row := range rows {
		grand += row.Total
	}

	result := make([]dto.CategoryBreakdown, 0, len(rows))
	for _, row := range rows {
		pct := 0.0
		if grand > 0 {
			pct = float64(int((row.Total/grand)*10000)) / 100
		}
		result = append(result, dto.CategoryBreakdown{
			CategoryName: row.CategoryName,
			Total:        row.Total,
			Percentage:   pct,
			TxCount:      row.TxCount,
		})
	}
	return result, nil
}