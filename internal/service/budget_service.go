package service

import (
	"context"
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/pkg/budgetcycle"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BudgetService interface {
	// GetCurrentBudgets mengembalikan budget periode yang sedang berjalan sesuai
	// tanggal mulai siklus user.
	GetCurrentBudgets(userID uuid.UUID) (*dto.BudgetSummaryResponse, error)
	CreateBudget(userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error)
	GetBudgetByID(userID uuid.UUID, id uuid.UUID) (*dto.BudgetResponse, error)
	GetAllBudgets(userID uuid.UUID, filter dto.BudgetFilterRequest) (*dto.BudgetSummaryResponse, error)
	UpdateBudget(userID uuid.UUID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error)
	DeleteBudget(userID uuid.UUID, id uuid.UUID) error
	CopyBudget(userID uuid.UUID, req dto.CopyBudgetRequest) ([]dto.BudgetResponse, error)
}

type budgetService struct {
	budgetRepo   repository.BudgetRepository
	categoryRepo repository.CategoryRepository
	userRepo     repository.UserRepository
	notifSvc     NotificationService
	goalRepo     repository.GoalRepository
	walletRepo   repository.WalletRepository
	exportRepo   repository.ExportRepository
	now          func() time.Time
}

func NewBudgetService(
	budgetRepo repository.BudgetRepository,
	categoryRepo repository.CategoryRepository,
	userRepo repository.UserRepository,
	notifSvc NotificationService,
	goalRepo repository.GoalRepository,
	walletRepo repository.WalletRepository,
	exportRepo repository.ExportRepository,
) BudgetService {
	return &budgetService{
		budgetRepo:   budgetRepo,
		categoryRepo: categoryRepo,
		userRepo:     userRepo,
		notifSvc:     notifSvc,
		goalRepo:     goalRepo,
		walletRepo:   walletRepo,
		exportRepo:   exportRepo,
		now:          time.Now,
	}
}

var jakarta = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*60*60)
}()

func (s *budgetService) startDay(userID uuid.UUID) (int, error) {
	day, err := s.userRepo.BudgetStartDay(context.Background(), userID)
	if err != nil {
		return 0, err
	}
	return budgetcycle.Normalize(day), nil
}

func (s *budgetService) GetCurrentBudgets(userID uuid.UUID) (*dto.BudgetSummaryResponse, error) {
	day, err := s.startDay(userID)
	if err != nil {
		return nil, err
	}
	year, month := budgetcycle.PeriodOf(day, s.now().In(jakarta))
	return s.GetAllBudgets(userID, dto.BudgetFilterRequest{Month: month, Year: year})
}

// ─── CreateBudget ─────────────────────────────────────────────────────────────

func (s *budgetService) CreateBudget(userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error) {
	budget := &model.Budget{
		UserID: userID,
		Kind:   model.BudgetExpense,
		Amount: req.Amount,
		Month:  req.Month,
		Year:   req.Year,
		Notes:  &req.Notes,
	}
	if req.Kind == model.BudgetSavings {
		if err := s.prepareSavings(userID, budget, req.GoalID, req.WalletID); err != nil {
			return nil, err
		}
	} else if err := s.prepareExpense(userID, budget, req.CategoryID); err != nil {
		return nil, err
	}

	if err := s.budgetRepo.Create(budget); err != nil {
		return nil, err
	}

	created, err := s.budgetRepo.FindByID(budget.ID, userID)
	if err != nil {
		return nil, err
	}

	resp, err := s.toBudgetResponse(userID, created)
	if err != nil {
		return nil, err
	}

	_ = s.notifSvc.CreateSystem(userID,
		"Anggaran Dibuat 🎯",
		fmt.Sprintf("Anggaran %s sebesar Rp %.0f berhasil dibuat untuk bulan %d/%d.",
			budgetName(resp), req.Amount, req.Month, req.Year),
	)

	return resp, nil
}

func (s *budgetService) prepareExpense(userID uuid.UUID, budget *model.Budget, categoryID *uuid.UUID) error {
	if categoryID == nil {
		return errors.New("kategori wajib dipilih untuk anggaran pengeluaran")
	}
	category, err := s.categoryRepo.FindByID(*categoryID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCategoryNotFound
	}
	if err != nil {
		return err
	}
	if category.Type != "expense" {
		return ErrBudgetCategoryNotExpense
	}
	existing, err := s.budgetRepo.FindByCategory(userID, *categoryID, budget.Month, budget.Year)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing != nil {
		return errors.New("anggaran untuk kategori ini sudah ada pada bulan dan tahun tersebut")
	}
	budget.CategoryID = categoryID
	return nil
}

// prepareSavings mengisi dompet tujuan dari target (bila dipilih) atau dari
// dompet yang dipilih langsung.
func (s *budgetService) prepareSavings(userID uuid.UUID, budget *model.Budget, goalID, walletID *uuid.UUID) error {
	budget.Kind = model.BudgetSavings
	if goalID != nil {
		goal, err := s.goalRepo.FindByID(*goalID, userID)
		if err != nil {
			return ErrGoalNotFound
		}
		budget.GoalID, walletID = &goal.ID, &goal.WalletID
	}
	if walletID == nil {
		return errors.New("pilih target tabungan atau dompet tabungan")
	}
	wallet, err := s.walletRepo.FindByID(*walletID, userID)
	if err != nil {
		return ErrWalletNotFound
	}
	if wallet.IsArchived {
		return ErrWalletArchived
	}
	if wallet.Type == model.WalletTypeCreditCard {
		return errors.New("kartu kredit tidak bisa dipakai sebagai dompet tabungan")
	}
	existing, err := s.budgetRepo.FindSavingsByWallet(userID, wallet.ID, budget.Month, budget.Year)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing != nil {
		return errors.New("anggaran tabungan untuk dompet ini sudah ada pada periode tersebut")
	}
	budget.WalletID = &wallet.ID
	return nil
}

func budgetName(b *dto.BudgetResponse) string {
	switch {
	case b.Goal != nil:
		return "tabungan " + b.Goal.Name
	case b.Wallet != nil:
		return "tabungan " + b.Wallet.Name
	default:
		return b.Category.Name
	}
}

// ─── GetBudgetByID ────────────────────────────────────────────────────────────

func (s *budgetService) GetBudgetByID(userID uuid.UUID, id uuid.UUID) (*dto.BudgetResponse, error) {
	budget, err := s.budgetRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("anggaran tidak ditemukan")
		}
		return nil, err
	}
	return s.toBudgetResponse(userID, budget)
}

// ─── GetAllBudgets ────────────────────────────────────────────────────────────

func (s *budgetService) GetAllBudgets(userID uuid.UUID, filter dto.BudgetFilterRequest) (*dto.BudgetSummaryResponse, error) {
	day, err := s.startDay(userID)
	if err != nil {
		return nil, err
	}

	budgets, err := s.budgetRepo.FindAll(userID, filter.Month, filter.Year)
	if err != nil {
		return nil, err
	}

	lookup := s.newBudgetLookup(userID, day)
	summary := &dto.BudgetSummaryResponse{Month: filter.Month, Year: filter.Year, StartDay: day, Budgets: []dto.BudgetResponse{}}
	for i := range budgets {
		item, err := s.buildBudgetResponse(userID, &budgets[i], day, lookup)
		if err != nil {
			return nil, err
		}
		summary.Budgets = append(summary.Budgets, *item)
		if item.Kind == model.BudgetSavings {
			summary.SavingsPlanned += item.Amount
			summary.SavingsDeposited += item.Spent
			continue
		}
		summary.TotalBudget += item.Amount
		summary.TotalSpent += item.Spent
	}
	summary.TotalRemain = summary.TotalBudget - summary.TotalSpent

	if filter.Month > 0 && filter.Year > 0 {
		summary.PeriodStart, summary.PeriodEnd = periodLabels(day, filter.Year, filter.Month)
		if err := s.addFreeMoney(userID, day, summary, lookup); err != nil {
			return nil, err
		}
	}
	return summary, nil
}

// ─── UpdateBudget ─────────────────────────────────────────────────────────────

func (s *budgetService) UpdateBudget(userID uuid.UUID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error) {
	budget, err := s.budgetRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("anggaran tidak ditemukan")
		}
		return nil, err
	}

	oldAmount := budget.Amount

	if req.Amount > 0 {
		budget.Amount = req.Amount
	}
	if req.Notes != "" {
		budget.Notes = &req.Notes
	}

	if err := s.budgetRepo.Update(budget); err != nil {
		return nil, err
	}

	updated, err := s.budgetRepo.FindByID(budget.ID, userID)
	if err != nil {
		return nil, err
	}

	resp, err := s.toBudgetResponse(userID, updated)
	if err != nil {
		return nil, err
	}

	// ── Notifikasi: anggaran diturunkan & sudah melebihi batas baru ───
	if resp.Kind == model.BudgetExpense && req.Amount > 0 && req.Amount < oldAmount && resp.UsedPct >= 100 {
		_ = s.notifSvc.CreateSystem(userID,
			"Anggaran Terlampaui Setelah Perubahan ❗",
			fmt.Sprintf("Anggaran %s dikurangi menjadi Rp %.0f, namun pengeluaran sudah Rp %.0f (%.0f%%).",
				updated.Category.Name, req.Amount, resp.Spent, resp.UsedPct),
		)
	}

	return resp, nil
}

// ─── DeleteBudget ─────────────────────────────────────────────────────────────

func (s *budgetService) DeleteBudget(userID uuid.UUID, id uuid.UUID) error {
	_, err := s.budgetRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("anggaran tidak ditemukan")
		}
		return err
	}
	return s.budgetRepo.Delete(id, userID)
}

// ─── CopyBudget ───────────────────────────────────────────────────────────────

func (s *budgetService) CopyBudget(userID uuid.UUID, req dto.CopyBudgetRequest) ([]dto.BudgetResponse, error) {
	if req.FromMonth == req.ToMonth && req.FromYear == req.ToYear {
		return nil, errors.New("bulan asal dan tujuan tidak boleh sama")
	}

	sources, err := s.budgetRepo.FindAll(userID, req.FromMonth, req.FromYear)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("tidak ada anggaran pada bulan asal")
	}

	var results []dto.BudgetResponse

	for _, src := range sources {
		var existing *model.Budget
		var err error
		if src.Kind == model.BudgetSavings {
			existing, err = s.budgetRepo.FindSavingsByWallet(userID, *src.WalletID, req.ToMonth, req.ToYear)
		} else {
			existing, err = s.budgetRepo.FindByCategory(userID, *src.CategoryID, req.ToMonth, req.ToYear)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if existing != nil {
			continue
		}

		newBudget := &model.Budget{
			UserID:     userID,
			Kind:       src.Kind,
			CategoryID: src.CategoryID,
			WalletID:   src.WalletID,
			GoalID:     src.GoalID,
			Amount:     src.Amount,
			Month:      req.ToMonth,
			Year:       req.ToYear,
			Notes:      src.Notes,
		}

		if err := s.budgetRepo.Create(newBudget); err != nil {
			return nil, err
		}

		created, err := s.budgetRepo.FindByID(newBudget.ID, userID)
		if err != nil {
			return nil, err
		}

		item, err := s.toBudgetResponse(userID, created)
		if err != nil {
			return nil, err
		}
		results = append(results, *item)
	}

	if results == nil {
		results = []dto.BudgetResponse{}
	}

	// ── Notifikasi: salin anggaran berhasil ───────────────────────────
	if len(results) > 0 {
		_ = s.notifSvc.CreateSystem(userID,
			"Anggaran Disalin 📋",
			fmt.Sprintf("%d anggaran berhasil disalin dari %d/%d ke %d/%d.",
				len(results), req.FromMonth, req.FromYear, req.ToMonth, req.ToYear),
		)
	}

	return results, nil
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func periodLabels(startDay, year, month int) (string, string) {
	start, end := budgetcycle.Period(startDay, year, month)
	return start.Format("2006-01-02"), end.AddDate(0, 0, -1).Format("2006-01-02")
}

func (s *budgetService) toBudgetResponse(userID uuid.UUID, b *model.Budget) (*dto.BudgetResponse, error) {
	day, err := s.startDay(userID)
	if err != nil {
		return nil, err
	}
	return s.buildBudgetResponse(userID, b, day, s.newBudgetLookup(userID, day))
}

func (s *budgetService) buildBudgetResponse(userID uuid.UUID, b *model.Budget, startDay int, lookup *budgetLookup) (*dto.BudgetResponse, error) {
	notes := ""
	if b.Notes != nil {
		notes = *b.Notes
	}
	resp := &dto.BudgetResponse{
		ID:     b.ID,
		Month:  b.Month,
		Year:   b.Year,
		Notes:  notes,
		Kind:   model.BudgetExpense,
		Amount: b.Amount,
	}
	resp.PeriodStart, resp.PeriodEnd = periodLabels(startDay, b.Year, b.Month)

	if b.Kind == model.BudgetSavings && b.WalletID != nil {
		deposited, err := lookup.deposited(b.Year, b.Month, *b.WalletID)
		if err != nil {
			return nil, err
		}
		resp.Kind = model.BudgetSavings
		resp.Spent = deposited
		resp.Remain = math.Max(b.Amount-deposited, 0)
		resp.UsedPct = percentOf(deposited, b.Amount)
		resp.Status = "in_progress"
		if deposited >= b.Amount {
			resp.Status = "achieved"
		}
		ref, err := lookup.wallet(*b.WalletID)
		if err != nil {
			return nil, err
		}
		resp.Wallet = ref
		if b.GoalID != nil {
			if resp.Goal, err = lookup.goal(*b.GoalID); err != nil {
				return nil, err
			}
		}
		return resp, nil
	}

	start, end := budgetcycle.Period(startDay, b.Year, b.Month)
	spent, err := s.budgetRepo.GetSpentByCategory(userID, *b.CategoryID, start, end)
	if err != nil {
		return nil, err
	}
	resp.Spent = spent
	resp.Remain = b.Amount - spent
	resp.UsedPct = percentOf(spent, b.Amount)
	resp.Status = budgetStatus(spent, b.Amount)
	resp.Category.ID = b.Category.ID
	resp.Category.Name = b.Category.Name
	return resp, nil
}

func percentOf(part, whole float64) float64 {
	if whole <= 0 {
		return 0
	}
	return math.Round((part/whole)*10000) / 100
}

func budgetStatus(spent, amount float64) string {
	switch {
	case spent > amount:
		return "over_budget"
	case spent == amount:
		return "used_up"
	case percentOf(spent, amount) >= 80:
		return "warning"
	default:
		return "on_track"
	}
}
