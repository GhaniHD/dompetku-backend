package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/repository"

	"github.com/google/uuid"
)

type ReportService interface {
	GetMonthlyReport(userID uuid.UUID, filter dto.ReportFilterRequest) (*dto.ReportResponse, error)
	GetYearlyReport(userID uuid.UUID, filter dto.YearlyReportFilterRequest) (*dto.YearlyReportResponse, error)
}

type reportService struct {
	reportRepo repository.ReportRepository
}

func NewReportService(reportRepo repository.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

// ─── Monthly ──────────────────────────────────────────────────────────────────

func (s *reportService) GetMonthlyReport(userID uuid.UUID, filter dto.ReportFilterRequest) (*dto.ReportResponse, error) {
	totalIncome, err := s.reportRepo.GetTotalByType(userID, "income", filter.Month, filter.Year)
	if err != nil {
		return nil, err
	}
	totalExpense, err := s.reportRepo.GetTotalByType(userID, "expense", filter.Month, filter.Year)
	if err != nil {
		return nil, err
	}
	incomeByCategory, err := s.reportRepo.GetBreakdownByCategory(userID, "income", filter.Month, filter.Year)
	if err != nil {
		return nil, err
	}
	expenseByCategory, err := s.reportRepo.GetBreakdownByCategory(userID, "expense", filter.Month, filter.Year)
	if err != nil {
		return nil, err
	}
	dailyTotals, err := s.reportRepo.GetDailyTotals(userID, filter.Month, filter.Year)
	if err != nil {
		return nil, err
	}

	if incomeByCategory == nil  { incomeByCategory  = []dto.CategoryBreakdown{} }
	if expenseByCategory == nil { expenseByCategory = []dto.CategoryBreakdown{} }
	if dailyTotals == nil       { dailyTotals       = []dto.DailyTotal{} }

	return &dto.ReportResponse{
		Month: filter.Month, Year: filter.Year,
		TotalIncome: totalIncome, TotalExpense: totalExpense,
		NetBalance:        totalIncome - totalExpense,
		IncomeByCategory:  incomeByCategory,
		ExpenseByCategory: expenseByCategory,
		DailyTotals:       dailyTotals,
	}, nil
}

// ─── Yearly ───────────────────────────────────────────────────────────────────

func (s *reportService) GetYearlyReport(userID uuid.UUID, filter dto.YearlyReportFilterRequest) (*dto.YearlyReportResponse, error) {
	totalIncome, err := s.reportRepo.GetTotalByTypeYearly(userID, "income", filter.Year)
	if err != nil {
		return nil, err
	}
	totalExpense, err := s.reportRepo.GetTotalByTypeYearly(userID, "expense", filter.Year)
	if err != nil {
		return nil, err
	}
	incomeByCategory, err := s.reportRepo.GetBreakdownByCategoryYearly(userID, "income", filter.Year)
	if err != nil {
		return nil, err
	}
	expenseByCategory, err := s.reportRepo.GetBreakdownByCategoryYearly(userID, "expense", filter.Year)
	if err != nil {
		return nil, err
	}
	monthlyBreakdown, err := s.reportRepo.GetMonthlyBreakdown(userID, filter.Year)
	if err != nil {
		return nil, err
	}

	if incomeByCategory == nil  { incomeByCategory  = []dto.CategoryBreakdown{} }
	if expenseByCategory == nil { expenseByCategory = []dto.CategoryBreakdown{} }
	if monthlyBreakdown == nil  { monthlyBreakdown  = []dto.MonthSummary{} }

	return &dto.YearlyReportResponse{
		Year:              filter.Year,
		TotalIncome:       totalIncome,
		TotalExpense:      totalExpense,
		NetBalance:        totalIncome - totalExpense,
		IncomeByCategory:  incomeByCategory,
		ExpenseByCategory: expenseByCategory,
		MonthlyBreakdown:  monthlyBreakdown,
	}, nil
}