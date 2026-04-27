package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BudgetService interface {
	CreateBudget(userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error)
	GetBudgetByID(userID uuid.UUID, id uuid.UUID) (*dto.BudgetResponse, error)
	GetAllBudgets(userID uuid.UUID, filter dto.BudgetFilterRequest) (*dto.BudgetSummaryResponse, error)
	UpdateBudget(userID uuid.UUID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error)
	DeleteBudget(userID uuid.UUID, id uuid.UUID) error
	CopyBudget(userID uuid.UUID, req dto.CopyBudgetRequest) ([]dto.BudgetResponse, error)
}

type budgetService struct {
	budgetRepo  repository.BudgetRepository
	notifSvc    NotificationService // ← inject notifikasi
}

func NewBudgetService(
	budgetRepo repository.BudgetRepository,
	notifSvc   NotificationService,
) BudgetService {
	return &budgetService{
		budgetRepo: budgetRepo,
		notifSvc:   notifSvc,
	}
}

// ─── CreateBudget ─────────────────────────────────────────────────────────────

func (s *budgetService) CreateBudget(userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error) {
	existing, err := s.budgetRepo.FindByCategory(userID, req.CategoryID, req.Month, req.Year)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("anggaran untuk kategori ini sudah ada pada bulan dan tahun tersebut")
	}

	budget := &model.Budget{
		UserID:     userID,
		CategoryID: req.CategoryID,
		Amount:     req.Amount,
		Month:      req.Month,
		Year:       req.Year,
		Notes:      &req.Notes,
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

	// ── Notifikasi: anggaran baru dibuat ──────────────────────────────
	_ = s.notifSvc.CreateSystem(userID,
		"Anggaran Dibuat 🎯",
		fmt.Sprintf("Anggaran %s sebesar Rp %.0f berhasil dibuat untuk bulan %d/%d.",
			created.Category.Name, req.Amount, req.Month, req.Year),
	)

	return resp, nil
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
	budgets, err := s.budgetRepo.FindAll(userID, filter.Month, filter.Year)
	if err != nil {
		return nil, err
	}

	var (
		items       []dto.BudgetResponse
		totalBudget float64
		totalSpent  float64
	)

	for _, b := range budgets {
		item, err := s.toBudgetResponse(userID, &b)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
		totalBudget += b.Amount
		totalSpent += item.Spent
	}

	if items == nil {
		items = []dto.BudgetResponse{}
	}

	summary := &dto.BudgetSummaryResponse{
		Month:       filter.Month,
		Year:        filter.Year,
		TotalBudget: totalBudget,
		TotalSpent:  totalSpent,
		TotalRemain: totalBudget - totalSpent,
		Budgets:     items,
	}

	// ── Notifikasi: total pengeluaran > 90% dari total anggaran bulan ini ──
	if totalBudget > 0 {
		overallPct := (totalSpent / totalBudget) * 100
		if overallPct >= 90 {
			_ = s.notifSvc.CreateSystem(userID,
				"Peringatan Anggaran Bulanan ⚠️",
				fmt.Sprintf("Total pengeluaran bulan %d/%d sudah mencapai %.0f%% dari total anggaran.",
					filter.Month, filter.Year, overallPct),
			)
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
	if req.Amount > 0 && req.Amount < oldAmount && resp.UsedPct >= 100 {
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
		existing, err := s.budgetRepo.FindByCategory(userID, src.CategoryID, req.ToMonth, req.ToYear)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if existing != nil {
			continue
		}

		newBudget := &model.Budget{
			UserID:     userID,
			CategoryID: src.CategoryID,
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

func (s *budgetService) toBudgetResponse(userID uuid.UUID, b *model.Budget) (*dto.BudgetResponse, error) {
	spent, err := s.budgetRepo.GetSpentByCategory(userID, b.CategoryID, b.Month, b.Year)
	if err != nil {
		return nil, err
	}

	remain := b.Amount - spent
	var usedPct float64
	if b.Amount > 0 {
		usedPct = math.Round((spent/b.Amount)*10000) / 100
	}

	status := budgetStatus(usedPct)

	notes := ""
	if b.Notes != nil {
		notes = *b.Notes
	}

	resp := &dto.BudgetResponse{
		ID:      b.ID,
		Month:   b.Month,
		Year:    b.Year,
		Notes:   notes,
		Amount:  b.Amount,
		Spent:   spent,
		Remain:  remain,
		UsedPct: usedPct,
		Status:  status,
	}
	resp.Category.ID = b.Category.ID
	resp.Category.Name = b.Category.Name

	return resp, nil
}

func budgetStatus(pct float64) string {
	switch {
	case pct >= 100:
		return "over_budget"
	case pct >= 80:
		return "warning"
	default:
		return "on_track"
	}
}