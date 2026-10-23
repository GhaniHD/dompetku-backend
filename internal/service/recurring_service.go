package service

import (
	"context"
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	recurringInterval   = time.Hour
	maxRecurringPerTick = 500
)

var errNoDueRecurring = errors.New("tidak ada jadwal yang jatuh tempo")

type RecurringService interface {
	Create(userID uuid.UUID, req dto.CreateRecurringRequest) (*dto.RecurringResponse, error)
	GetAll(userID uuid.UUID) ([]dto.RecurringResponse, error)
	GetByID(userID uuid.UUID, id uuid.UUID) (*dto.RecurringResponse, error)
	Update(userID uuid.UUID, id uuid.UUID, req dto.UpdateRecurringRequest) (*dto.RecurringResponse, error)
	Delete(userID uuid.UUID, id uuid.UUID) error
	// ProcessDue mencatat semua jadwal yang jatuh tempo sampai tanggal `now`,
	// termasuk yang terlewat saat server mati. Aman dijalankan paralel.
	ProcessDue(now time.Time) (int, error)
}

type recurringService struct {
	db              *gorm.DB
	repo            repository.RecurringRepository
	walletRepo      repository.WalletRepository
	transactionRepo repository.TransactionRepository
	alerter         *Alerter
	notif           NotificationService
	now             func() time.Time
}

func NewRecurringService(
	db *gorm.DB,
	repo repository.RecurringRepository,
	walletRepo repository.WalletRepository,
	transactionRepo repository.TransactionRepository,
	alerter *Alerter,
	notif NotificationService,
) RecurringService {
	return &recurringService{
		db:              db,
		repo:            repo,
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		alerter:         alerter,
		notif:           notif,
		now:             time.Now,
	}
}

func today(now time.Time) time.Time {
	t := now.In(jakarta)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *recurringService) ensureUsableWallet(userID, walletID uuid.UUID) error {
	wallet, err := s.walletRepo.FindByID(walletID, userID)
	if err != nil {
		return err
	}
	if wallet.IsArchived {
		return ErrWalletArchived
	}
	return nil
}

func (s *recurringService) Create(userID uuid.UUID, req dto.CreateRecurringRequest) (*dto.RecurringResponse, error) {
	start := dateOnlyUTC(req.StartDate)
	if req.EndDate != nil && dateOnlyUTC(*req.EndDate).Before(start) {
		return nil, ErrRecurringEndBeforeStart
	}
	if err := s.ensureUsableWallet(userID, req.WalletID); err != nil {
		return nil, err
	}
	if err := ensureCategory(s.db, userID, req.CategoryID, req.Type); err != nil {
		return nil, err
	}

	rule := &model.RecurringTransaction{
		UserID:      userID,
		WalletID:    req.WalletID,
		CategoryID:  req.CategoryID,
		Type:        req.Type,
		Amount:      roundMoney(req.Amount),
		Note:        req.Note,
		Frequency:   req.Frequency,
		AnchorDay:   start.Day(),
		StartDate:   start,
		NextRunDate: start,
		IsActive:    true,
	}
	if req.EndDate != nil {
		end := dateOnlyUTC(*req.EndDate)
		rule.EndDate = &end
	}
	if err := s.repo.Create(rule); err != nil {
		return nil, err
	}

	if _, err := s.processRule(rule.ID, s.now()); err != nil {
		log.Printf("[recurring] gagal memproses jadwal baru %s: %v", rule.ID, err)
	}
	return s.GetByID(userID, rule.ID)
}

func (s *recurringService) GetAll(userID uuid.UUID) ([]dto.RecurringResponse, error) {
	rules, err := s.repo.FindAll(userID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.RecurringResponse, 0, len(rules))
	for i := range rules {
		result = append(result, *toRecurringResponse(&rules[i]))
	}
	return result, nil
}

func (s *recurringService) GetByID(userID uuid.UUID, id uuid.UUID) (*dto.RecurringResponse, error) {
	rule, err := s.repo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRecurringNotFound
	}
	if err != nil {
		return nil, err
	}
	return toRecurringResponse(rule), nil
}

func (s *recurringService) Update(userID uuid.UUID, id uuid.UUID, req dto.UpdateRecurringRequest) (*dto.RecurringResponse, error) {
	rule, err := s.repo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRecurringNotFound
	}
	if err != nil {
		return nil, err
	}

	if req.WalletID != nil && *req.WalletID != rule.WalletID {
		if err := s.ensureUsableWallet(userID, *req.WalletID); err != nil {
			return nil, err
		}
		rule.WalletID = *req.WalletID
	}
	if req.CategoryID != nil && *req.CategoryID != rule.CategoryID {
		if err := ensureCategory(s.db, userID, *req.CategoryID, rule.Type); err != nil {
			return nil, err
		}
		rule.CategoryID = *req.CategoryID
	}
	if req.Amount != nil {
		rule.Amount = roundMoney(*req.Amount)
	}
	if req.Note != nil {
		rule.Note = *req.Note
	}
	if req.EndDate != nil {
		end := dateOnlyUTC(*req.EndDate)
		if end.Before(rule.StartDate) {
			return nil, ErrRecurringEndBeforeStart
		}
		rule.EndDate = &end
	}
	if req.IsActive != nil {
		// Saat diaktifkan lagi, jadwal yang terlewat selama dijeda tidak dicatat mundur.
		if *req.IsActive && !rule.IsActive {
			for t := today(s.now()); rule.NextRunDate.Before(t); {
				rule.NextRunDate = rule.NextAfter(rule.NextRunDate)
			}
		}
		rule.IsActive = *req.IsActive
	}

	if err := s.repo.Update(rule); err != nil {
		return nil, err
	}
	if rule.IsActive {
		if _, err := s.processRule(rule.ID, s.now()); err != nil {
			log.Printf("[recurring] gagal memproses jadwal %s: %v", rule.ID, err)
		}
	}
	return s.GetByID(userID, id)
}

func (s *recurringService) Delete(userID uuid.UUID, id uuid.UUID) error {
	affected, err := s.repo.Delete(id, userID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrRecurringNotFound
	}
	return nil
}

func (s *recurringService) ProcessDue(now time.Time) (int, error) {
	count := 0
	for i := 0; i < maxRecurringPerTick; i++ {
		done, err := s.processOne(today(now), nil)
		if err != nil {
			return count, err
		}
		if !done {
			break
		}
		count++
	}
	return count, nil
}

func (s *recurringService) processRule(ruleID uuid.UUID, now time.Time) (int, error) {
	count := 0
	for i := 0; i < maxRecurringPerTick; i++ {
		done, err := s.processOne(today(now), &ruleID)
		if err != nil {
			return count, err
		}
		if !done {
			break
		}
		count++
	}
	return count, nil
}

func isBusinessError(err error) bool {
	for _, target := range []error{
		ErrInsufficientBalance, ErrWalletArchived, ErrWalletNotFound,
		ErrCategoryNotFound, ErrCategoryTypeMismatch,
	} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// processOne mencatat satu kejadian jadwal yang jatuh tempo. Baris jadwal dikunci
// dengan FOR UPDATE SKIP LOCKED sehingga dua server tidak mencatat kejadian yang
// sama. Bila transaksi gagal karena aturan bisnis (misalnya saldo tidak cukup),
// jadwal tetap dimajukan dan user diberi notifikasi, agar tidak macet selamanya.
func (s *recurringService) processOne(today time.Time, ruleID *uuid.UUID) (bool, error) {
	var (
		rule    model.RecurringTransaction
		runDate time.Time
		created *createdTransaction
		failure error
	)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		q := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("is_active = true AND next_run_date <= ?", today.Format("2006-01-02"))
		if ruleID != nil {
			q = q.Where("id = ?", *ruleID)
		}
		res := q.Order("next_run_date ASC, id ASC").Limit(1).Find(&rule)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errNoDueRecurring
		}

		runDate = dateOnlyUTC(rule.NextRunDate)
		if !rule.Finished(runDate) {
			if err := tx.SavePoint("occurrence").Error; err != nil {
				return err
			}
			var err error
			created, err = createTransactionTx(tx, rule.UserID, dto.CreateTransactionRequest{
				WalletID:   rule.WalletID,
				CategoryID: rule.CategoryID,
				Amount:     rule.Amount,
				Type:       rule.Type,
				Note:       rule.Note,
				Date:       runDate,
			}, &rule.ID)
			if err != nil {
				if !isBusinessError(err) {
					return err
				}
				if err := tx.RollbackTo("occurrence").Error; err != nil {
					return err
				}
				created, failure = nil, err
			}
		}

		ranAt := time.Now()
		rule.LastRunAt = &ranAt
		rule.LastError = ""
		if failure != nil {
			rule.LastError = failure.Error()
		}
		rule.NextRunDate = rule.NextAfter(runDate)
		if rule.Finished(rule.NextRunDate) || rule.Finished(runDate) {
			rule.IsActive = false
		}

		return tx.Model(&rule).
			Select("next_run_date", "last_run_at", "last_error", "is_active").
			Updates(&rule).Error
	})
	if errors.Is(err, errNoDueRecurring) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if created != nil {
		if t, err := s.transactionRepo.FindByID(created.ID, rule.UserID); err == nil {
			s.alerter.afterCreate(rule.UserID, created, t)
		}
	}
	if failure != nil {
		_ = s.notif.CreateSystem(rule.UserID,
			"Transaksi Berulang Gagal Dicatat ⚠️",
			fmt.Sprintf("Jadwal Rp %.0f tanggal %s tidak tercatat: %s",
				rule.Amount, runDate.Format("02-01-2006"), failure.Error()),
		)
	}
	return true, nil
}

// StartScheduler menjalankan transaksi berulang dan pengingat jatuh tempo hutang
// saat server start lalu setiap jam.
func StartScheduler(lc fx.Lifecycle, svc RecurringService, debts DebtService) {
	ctx, cancel := context.WithCancel(context.Background())

	run := func() {
		count, err := svc.ProcessDue(time.Now())
		if err != nil {
			log.Printf("[recurring] gagal memproses jadwal: %v", err)
		}
		if count > 0 {
			log.Printf("[recurring] %d transaksi berulang dicatat", count)
		}
		if _, err := debts.SendDueReminders(time.Now()); err != nil {
			log.Printf("[debt] gagal mengirim pengingat: %v", err)
		}
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				run()
				ticker := time.NewTicker(recurringInterval)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						run()
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func dateOnlyUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func toRecurringResponse(r *model.RecurringTransaction) *dto.RecurringResponse {
	return &dto.RecurringResponse{
		ID:          r.ID,
		Wallet:      dto.TransferWallet{ID: r.Wallet.ID, Name: r.Wallet.Name},
		Category:    dto.TransferWallet{ID: r.Category.ID, Name: r.Category.Name},
		Type:        r.Type,
		Amount:      r.Amount,
		Note:        r.Note,
		Frequency:   r.Frequency,
		StartDate:   r.StartDate.Format("2006-01-02"),
		EndDate:     formatDate(r.EndDate),
		NextRunDate: r.NextRunDate.Format("2006-01-02"),
		IsActive:    r.IsActive,
		LastRunAt:   r.LastRunAt,
		LastError:   r.LastError,
	}
}
