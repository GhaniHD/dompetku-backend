package repository

import (
	"dompetku/internal/dto"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReportRepository interface {
	GetTotalByType(userID uuid.UUID, txType string, month, year int) (float64, error)
	GetBreakdownByCategory(userID uuid.UUID, txType string, month, year int) ([]dto.CategoryBreakdown, error)
	GetDailyTotals(userID uuid.UUID, month, year int) ([]dto.DailyTotal, error)

	// Yearly
	GetTotalByTypeYearly(userID uuid.UUID, txType string, year int) (float64, error)
	GetBreakdownByCategoryYearly(userID uuid.UUID, txType string, year int) ([]dto.CategoryBreakdown, error)
	GetMonthlyBreakdown(userID uuid.UUID, year int) ([]dto.MonthSummary, error)
}

type reportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) ReportRepository {
	return &reportRepository{db: db}
}

// ─── Monthly ──────────────────────────────────────────────────────────────────

func (r *reportRepository) GetTotalByType(userID uuid.UUID, txType string, month, year int) (float64, error) {
	var total float64
	err := r.db.Table("transactions").
		Select("COALESCE(SUM(amount), 0)").
		Where(`user_id = ? AND type = ?
			AND EXTRACT(MONTH FROM transaction_date) = ?
			AND EXTRACT(YEAR  FROM transaction_date) = ?
			AND deleted_at IS NULL`, userID, txType, month, year).
		Scan(&total).Error
	return total, err
}

func (r *reportRepository) GetBreakdownByCategory(userID uuid.UUID, txType string, month, year int) ([]dto.CategoryBreakdown, error) {
	type rawRow struct {
		CategoryID   uuid.UUID `gorm:"column:category_id"`
		CategoryName string    `gorm:"column:category_name"`
		Total        float64   `gorm:"column:total"`
		TxCount      int       `gorm:"column:tx_count"`
	}
	var rows []rawRow
	err := r.db.Table("transactions t").
		Select("t.category_id, c.name AS category_name, COALESCE(SUM(t.amount),0) AS total, COUNT(t.id) AS tx_count").
		Joins("JOIN categories c ON c.id = t.category_id").
		Where(`t.user_id = ? AND t.type = ?
			AND EXTRACT(MONTH FROM t.transaction_date) = ?
			AND EXTRACT(YEAR  FROM t.transaction_date) = ?
			AND t.deleted_at IS NULL`, userID, txType, month, year).
		Group("t.category_id, c.name").Order("total DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toBreakdown(rows, func(r rawRow) (uuid.UUID, string, float64, int) {
		return r.CategoryID, r.CategoryName, r.Total, r.TxCount
	}), nil
}

func (r *reportRepository) GetDailyTotals(userID uuid.UUID, month, year int) ([]dto.DailyTotal, error) {
	type rawRow struct {
		Date    string  `gorm:"column:date"`
		Income  float64 `gorm:"column:income"`
		Expense float64 `gorm:"column:expense"`
	}
	var rows []rawRow
	err := r.db.Table("transactions").
		Select(`TO_CHAR(transaction_date,'YYYY-MM-DD') AS date,
			COALESCE(SUM(CASE WHEN type='income'  THEN amount ELSE 0 END),0) AS income,
			COALESCE(SUM(CASE WHEN type='expense' THEN amount ELSE 0 END),0) AS expense`).
		Where(`user_id = ?
			AND EXTRACT(MONTH FROM transaction_date) = ?
			AND EXTRACT(YEAR  FROM transaction_date) = ?
			AND deleted_at IS NULL`, userID, month, year).
		Group("date").Order("date ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]dto.DailyTotal, 0, len(rows))
	for _, row := range rows {
		result = append(result, dto.DailyTotal{Date: row.Date, Income: row.Income, Expense: row.Expense})
	}
	return result, nil
}

// ─── Yearly ───────────────────────────────────────────────────────────────────

func (r *reportRepository) GetTotalByTypeYearly(userID uuid.UUID, txType string, year int) (float64, error) {
	var total float64
	err := r.db.Table("transactions").
		Select("COALESCE(SUM(amount), 0)").
		Where(`user_id = ? AND type = ?
			AND EXTRACT(YEAR FROM transaction_date) = ?
			AND deleted_at IS NULL`, userID, txType, year).
		Scan(&total).Error
	return total, err
}

func (r *reportRepository) GetBreakdownByCategoryYearly(userID uuid.UUID, txType string, year int) ([]dto.CategoryBreakdown, error) {
	type rawRow struct {
		CategoryID   uuid.UUID `gorm:"column:category_id"`
		CategoryName string    `gorm:"column:category_name"`
		Total        float64   `gorm:"column:total"`
		TxCount      int       `gorm:"column:tx_count"`
	}
	var rows []rawRow
	err := r.db.Table("transactions t").
		Select("t.category_id, c.name AS category_name, COALESCE(SUM(t.amount),0) AS total, COUNT(t.id) AS tx_count").
		Joins("JOIN categories c ON c.id = t.category_id").
		Where(`t.user_id = ? AND t.type = ?
			AND EXTRACT(YEAR FROM t.transaction_date) = ?
			AND t.deleted_at IS NULL`, userID, txType, year).
		Group("t.category_id, c.name").Order("total DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toBreakdown(rows, func(r rawRow) (uuid.UUID, string, float64, int) {
		return r.CategoryID, r.CategoryName, r.Total, r.TxCount
	}), nil
}

func (r *reportRepository) GetMonthlyBreakdown(userID uuid.UUID, year int) ([]dto.MonthSummary, error) {
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
		Where(`user_id = ?
			AND EXTRACT(YEAR FROM transaction_date) = ?
			AND deleted_at IS NULL`, userID, year).
		Group("month").Order("month ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]dto.MonthSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, dto.MonthSummary{
			Month:   row.Month,
			Income:  row.Income,
			Expense: row.Expense,
			Net:     row.Income - row.Expense,
		})
	}
	return result, nil
}

// ─── Helper ───────────────────────────────────────────────────────────────────

type rawBreakdown interface {
	comparable
}

func toBreakdown[T any](rows []T, extract func(T) (uuid.UUID, string, float64, int)) []dto.CategoryBreakdown {
	var grand float64
	for _, row := range rows {
		_, _, total, _ := extract(row)
		grand += total
	}
	result := make([]dto.CategoryBreakdown, 0, len(rows))
	for _, row := range rows {
		id, name, total, count := extract(row)
		pct := 0.0
		if grand > 0 {
			pct = float64(int((total/grand)*10000)) / 100
		}
		result = append(result, dto.CategoryBreakdown{
			CategoryID: id, CategoryName: name,
			Total: total, Percentage: pct, TxCount: count,
		})
	}
	return result
}