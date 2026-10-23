package service

import (
	"context"
	"dompetku/internal/dto"
	"dompetku/internal/repository"
	"dompetku/pkg/budgetcycle"
	"time"

	"github.com/google/uuid"
)

type ReportService interface {
	GetMonthlyReport(userID uuid.UUID, filter dto.ReportFilterRequest) (*dto.ReportResponse, error)
	GetYearlyReport(userID uuid.UUID, filter dto.YearlyReportFilterRequest) (*dto.YearlyReportResponse, error)
}

type reportService struct {
	reportRepo repository.ReportRepository
	userRepo   repository.UserRepository
}

func NewReportService(reportRepo repository.ReportRepository, userRepo repository.UserRepository) ReportService {
	return &reportService{reportRepo: reportRepo, userRepo: userRepo}
}

func (s *reportService) monthRange(userID uuid.UUID, filter dto.ReportFilterRequest) (start, end time.Time, period string, err error) {
	if filter.Period != "cycle" {
		start, end = budgetcycle.Period(1, filter.Year, filter.Month)
		return start, end, "calendar", nil
	}
	startDay, err := s.userRepo.BudgetStartDay(context.Background(), userID)
	if err != nil {
		return start, end, "", err
	}
	start, end = budgetcycle.Period(budgetcycle.Normalize(startDay), filter.Year, filter.Month)
	return start, end, "cycle", nil
}

// ─── Monthly ──────────────────────────────────────────────────────────────────

func (s *reportService) GetMonthlyReport(userID uuid.UUID, filter dto.ReportFilterRequest) (*dto.ReportResponse, error) {
	start, end, period, err := s.monthRange(userID, filter)
	if err != nil {
		return nil, err
	}

	totalIncome, err := s.reportRepo.GetTotalByType(userID, "income", start, end)
	if err != nil {
		return nil, err
	}
	totalExpense, err := s.reportRepo.GetTotalByType(userID, "expense", start, end)
	if err != nil {
		return nil, err
	}
	incomeByCategory, err := s.reportRepo.GetBreakdownByCategory(userID, "income", start, end)
	if err != nil {
		return nil, err
	}
	expenseByCategory, err := s.reportRepo.GetBreakdownByCategory(userID, "expense", start, end)
	if err != nil {
		return nil, err
	}
	dailyTotals, err := s.reportRepo.GetDailyTotals(userID, start, end)
	if err != nil {
		return nil, err
	}

	if incomeByCategory == nil {
		incomeByCategory = []dto.CategoryBreakdown{}
	}
	if expenseByCategory == nil {
		expenseByCategory = []dto.CategoryBreakdown{}
	}
	if dailyTotals == nil {
		dailyTotals = []dto.DailyTotal{}
	}

	return &dto.ReportResponse{
		Month: filter.Month, Year: filter.Year,
		Period:      period,
		PeriodStart: start.Format("2006-01-02"),
		PeriodEnd:   end.AddDate(0, 0, -1).Format("2006-01-02"),
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

	if incomeByCategory == nil {
		incomeByCategory = []dto.CategoryBreakdown{}
	}
	if expenseByCategory == nil {
		expenseByCategory = []dto.CategoryBreakdown{}
	}
	if monthlyBreakdown == nil {
		monthlyBreakdown = []dto.MonthSummary{}
	}

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
