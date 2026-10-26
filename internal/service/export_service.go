package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/pkg/budgetcycle"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

const maxExportDays = 3 * 366

var (
	ErrExportRange    = errors.New("tanggal akhir tidak boleh sebelum tanggal awal")
	ErrExportTooLarge = errors.New("rentang laporan maksimal 3 tahun")
)

type ExportService interface {
	// Workbook menyusun laporan Excel untuk rentang tanggal inklusif. Bila start
	// atau end nil, dipakai periode gajian yang sedang berjalan.
	Workbook(userID uuid.UUID, start, end *time.Time) (*excelize.File, string, error)
	// SingleSheet menyusun satu sheet rapi (transaksi atau pindahan) dengan
	// tabel berfilter dan ringkasan SUBTOTAL.
	SingleSheet(userID uuid.UUID, kind string, start, end *time.Time) (*excelize.File, string, error)
}

type exportService struct {
	repo         repository.ExportRepository
	userRepo     repository.UserRepository
	walletRepo   repository.WalletRepository
	transferRepo repository.TransferRepository
	budgets      BudgetService
	debts        DebtService
	goals        GoalService
	recurring    RecurringService
	now          func() time.Time
}

func NewExportService(
	repo repository.ExportRepository,
	userRepo repository.UserRepository,
	walletRepo repository.WalletRepository,
	transferRepo repository.TransferRepository,
	budgets BudgetService,
	debts DebtService,
	goals GoalService,
	recurring RecurringService,
) ExportService {
	return &exportService{
		repo: repo, userRepo: userRepo, walletRepo: walletRepo, transferRepo: transferRepo,
		budgets: budgets, debts: debts, goals: goals, recurring: recurring, now: time.Now,
	}
}

// reportData adalah semua bahan laporan; dirender oleh export_workbook.go.
type reportData struct {
	User         *model.User
	Start, End   time.Time
	GeneratedAt  time.Time
	Wallets      []model.Wallet
	WalletName   map[uuid.UUID]string
	OpeningByID  map[uuid.UUID]float64
	Entries      []repository.ExportEntry
	Transfers    []model.Transfer
	Budgets      []dto.BudgetSummaryResponse
	Debts        *dto.DebtSummaryResponse
	DebtMoves    []repository.ExportDebtMovement
	Goals        []dto.GoalResponse
	Recurring    []dto.RecurringResponse
	BudgetCycleD int
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *exportService) resolveRange(userID uuid.UUID, start, end *time.Time) (time.Time, time.Time, int, error) {
	day, err := s.userRepo.BudgetStartDay(context.Background(), userID)
	if err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	day = budgetcycle.Normalize(day)

	if start == nil || end == nil {
		y, m := budgetcycle.PeriodOf(day, s.now().In(jakarta))
		ps, pe := budgetcycle.Period(day, y, m)
		if start == nil {
			start = &ps
		}
		if end == nil {
			last := pe.AddDate(0, 0, -1)
			end = &last
		}
	}

	from, to := dateOnly(*start), dateOnly(*end)
	if to.Before(from) {
		return time.Time{}, time.Time{}, 0, ErrExportRange
	}
	if to.Sub(from).Hours()/24 > maxExportDays {
		return time.Time{}, time.Time{}, 0, ErrExportTooLarge
	}
	return from, to, day, nil
}

func (s *exportService) Workbook(userID uuid.UUID, start, end *time.Time) (*excelize.File, string, error) {
	from, to, startDay, err := s.resolveRange(userID, start, end)
	if err != nil {
		return nil, "", err
	}
	startStr, endStr := from.Format("2006-01-02"), to.Format("2006-01-02")

	user, err := s.userRepo.FindByID(context.Background(), userID)
	if err != nil || user == nil {
		return nil, "", ErrUserNotFound
	}

	wallets, err := s.walletRepo.FindAll(userID, true)
	if err != nil {
		return nil, "", err
	}
	sort.SliceStable(wallets, func(i, j int) bool {
		if wallets[i].IsArchived != wallets[j].IsArchived {
			return !wallets[i].IsArchived
		}
		return wallets[i].Name < wallets[j].Name
	})
	names := make(map[uuid.UUID]string, len(wallets))
	for _, w := range wallets {
		names[w.ID] = w.Name
	}

	opening, err := s.repo.BalancesBefore(userID, startStr)
	if err != nil {
		return nil, "", err
	}
	entries, err := s.repo.Entries(userID, startStr, endStr)
	if err != nil {
		return nil, "", err
	}

	endOfDay := to.Add(24*time.Hour - time.Second)
	transfers, err := s.transferRepo.FindAll(userID, dto.TransferFilterRequest{StartDate: &from, EndDate: &endOfDay})
	if err != nil {
		return nil, "", err
	}
	sort.SliceStable(transfers, func(i, j int) bool {
		if !transfers[i].TransferDate.Equal(transfers[j].TransferDate) {
			return transfers[i].TransferDate.Before(transfers[j].TransferDate)
		}
		return transfers[i].CreatedAt.Before(transfers[j].CreatedAt)
	})

	var budgets []dto.BudgetSummaryResponse
	for y, m := budgetcycle.PeriodOf(startDay, from); ; {
		ps, _ := budgetcycle.Period(startDay, y, m)
		if ps.After(to) {
			break
		}
		summary, err := s.budgets.GetAllBudgets(userID, dto.BudgetFilterRequest{Month: m, Year: y})
		if err != nil {
			return nil, "", err
		}
		if len(summary.Budgets) > 0 {
			budgets = append(budgets, *summary)
		}
		if m++; m > 12 {
			m, y = 1, y+1
		}
	}

	debts, err := s.debts.GetAll(userID, dto.DebtFilterRequest{})
	if err != nil {
		return nil, "", err
	}
	debtMoves, err := s.repo.DebtMovements(userID, startStr, endStr)
	if err != nil {
		return nil, "", err
	}
	goals, err := s.goals.GetAll(userID)
	if err != nil {
		return nil, "", err
	}
	recurring, err := s.recurring.GetAll(userID)
	if err != nil {
		return nil, "", err
	}

	data := &reportData{
		User: user, Start: from, End: to, GeneratedAt: s.now().In(jakarta),
		Wallets: wallets, WalletName: names, OpeningByID: opening, Entries: entries,
		Transfers: transfers, Budgets: budgets, Debts: debts, DebtMoves: debtMoves,
		Goals: goals, Recurring: recurring, BudgetCycleD: startDay,
	}

	f, err := renderWorkbook(data)
	if err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("dompetku-laporan_%s_%s.xlsx", startStr, endStr)
	return f, name, nil
}
