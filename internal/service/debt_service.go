package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DebtService interface {
	Create(userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error)
	GetAll(userID uuid.UUID, filter dto.DebtFilterRequest) (*dto.DebtSummaryResponse, error)
	GetByID(userID uuid.UUID, id uuid.UUID) (*dto.DebtResponse, error)
	Update(userID uuid.UUID, id uuid.UUID, req dto.UpdateDebtRequest) (*dto.DebtResponse, error)
	Delete(userID uuid.UUID, id uuid.UUID) error
	Pay(userID uuid.UUID, id uuid.UUID, req dto.DebtPaymentRequest) (*dto.DebtResponse, error)
	DeletePayment(userID uuid.UUID, id uuid.UUID, paymentID uuid.UUID) (*dto.DebtResponse, error)
	SendDueReminders(now time.Time) (int, error)
}

type debtService struct {
	db      *gorm.DB
	repo    repository.DebtRepository
	alerter *Alerter
	notif   NotificationService
	now     func() time.Time
}

func NewDebtService(db *gorm.DB, repo repository.DebtRepository, alerter *Alerter, notif NotificationService) DebtService {
	return &debtService{db: db, repo: repo, alerter: alerter, notif: notif, now: time.Now}
}

func (s *debtService) dateOr(t *time.Time) time.Time {
	if t != nil {
		return dateOnlyUTC(*t)
	}
	return today(s.now())
}

// moveWallet mencatat pergerakan hutang dan, bila ada wallet, mengubah saldonya
// lewat ledger sehingga aturan saldo minus dan arsip tetap berlaku.
func moveWallet(tx *gorm.DB, userID uuid.UUID, walletID *uuid.UUID, delta float64) (*walletLedger, error) {
	if walletID == nil {
		return nil, nil
	}
	ledger, err := lockLedger(tx, userID, *walletID)
	if err != nil {
		return nil, err
	}
	if err := ledger.requireUsable(*walletID); err != nil {
		return nil, err
	}
	return ledger, ledger.apply(*walletID, delta)
}

func (s *debtService) Create(userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error) {
	amount := roundMoney(req.Amount)
	debt := &model.Debt{
		UserID:       userID,
		Kind:         req.Kind,
		Counterparty: req.Counterparty,
		Principal:    amount,
		Note:         req.Note,
	}
	if req.DueDate != nil {
		due := dateOnlyUTC(*req.DueDate)
		debt.DueDate = &due
	}

	var ledger *walletLedger
	err := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewDebtRepository(tx)
		if err := repo.Create(debt); err != nil {
			return err
		}

		delta := 0.0
		if req.WalletID != nil {
			delta = debt.WalletSign() * amount
		}
		var err error
		if ledger, err = moveWallet(tx, userID, req.WalletID, delta); err != nil {
			return err
		}

		return repo.CreateMovement(&model.DebtMovement{
			DebtID:       debt.ID,
			UserID:       userID,
			WalletID:     req.WalletID,
			Kind:         model.DebtMovementPrincipal,
			Amount:       amount,
			WalletDelta:  delta,
			MovementDate: s.dateOr(req.Date),
			Note:         req.Note,
		})
	})
	if err != nil {
		return nil, err
	}

	s.alerter.BalanceChanges(userID, ledger.changes())
	return s.GetByID(userID, debt.ID)
}

func (s *debtService) GetAll(userID uuid.UUID, filter dto.DebtFilterRequest) (*dto.DebtSummaryResponse, error) {
	debts, err := s.repo.FindAll(userID, filter)
	if err != nil {
		return nil, err
	}

	summary := &dto.DebtSummaryResponse{Debts: make([]dto.DebtResponse, 0, len(debts))}
	for i := range debts {
		d := &debts[i]
		if d.Kind == model.DebtPayable {
			summary.TotalPayable += d.Remaining()
		} else {
			summary.TotalReceivable += d.Remaining()
		}
		summary.Debts = append(summary.Debts, *s.toResponse(d, false))
	}
	return summary, nil
}

func (s *debtService) GetByID(userID uuid.UUID, id uuid.UUID) (*dto.DebtResponse, error) {
	debt, err := s.repo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDebtNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.toResponse(debt, true), nil
}

func (s *debtService) Update(userID uuid.UUID, id uuid.UUID, req dto.UpdateDebtRequest) (*dto.DebtResponse, error) {
	debt, err := s.repo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDebtNotFound
	}
	if err != nil {
		return nil, err
	}

	if req.Counterparty != nil {
		debt.Counterparty = *req.Counterparty
	}
	if req.Note != nil {
		debt.Note = *req.Note
	}
	if req.DueDate != nil {
		due := dateOnlyUTC(*req.DueDate)
		debt.DueDate = &due
		debt.LastRemindedOn = nil
	}
	if err := s.repo.Update(debt); err != nil {
		return nil, err
	}
	return s.GetByID(userID, id)
}

func (s *debtService) Pay(userID uuid.UUID, id uuid.UUID, req dto.DebtPaymentRequest) (*dto.DebtResponse, error) {
	amount := roundMoney(req.Amount)
	var (
		ledger  *walletLedger
		debt    *model.Debt
		paidOff bool
	)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewDebtRepository(tx)
		var err error
		debt, err = repo.FindByIDForUpdate(id, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDebtNotFound
		}
		if err != nil {
			return err
		}
		if amount > roundMoney(debt.Remaining())+balanceEpsilon {
			return ErrDebtOverpaid
		}

		delta := 0.0
		if req.WalletID != nil {
			delta = -debt.WalletSign() * amount
		}
		if ledger, err = moveWallet(tx, userID, req.WalletID, delta); err != nil {
			return err
		}

		if err := repo.CreateMovement(&model.DebtMovement{
			DebtID:       debt.ID,
			UserID:       userID,
			WalletID:     req.WalletID,
			Kind:         model.DebtMovementPayment,
			Amount:       amount,
			WalletDelta:  delta,
			MovementDate: s.dateOr(req.Date),
			Note:         req.Note,
		}); err != nil {
			return err
		}

		debt.PaidAmount = roundMoney(debt.PaidAmount + amount)
		paidOff = math.Abs(debt.Remaining()) < balanceEpsilon
		return repo.Update(debt)
	})
	if err != nil {
		return nil, err
	}

	s.alerter.BalanceChanges(userID, ledger.changes())
	if paidOff {
		title, message := "Hutang Lunas ✅", fmt.Sprintf("Hutang ke %s sebesar Rp %.0f sudah lunas.", debt.Counterparty, debt.Principal)
		if debt.Kind == model.DebtReceivable {
			title, message = "Piutang Lunas ✅", fmt.Sprintf("Piutang dari %s sebesar Rp %.0f sudah dilunasi.", debt.Counterparty, debt.Principal)
		}
		_ = s.notif.CreateSystem(userID, title, message)
	}
	return s.GetByID(userID, id)
}

func (s *debtService) DeletePayment(userID uuid.UUID, id uuid.UUID, paymentID uuid.UUID) (*dto.DebtResponse, error) {
	var ledger *walletLedger

	err := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewDebtRepository(tx)
		debt, err := repo.FindByIDForUpdate(id, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDebtNotFound
		}
		if err != nil {
			return err
		}
		movement, err := repo.FindMovement(paymentID, debt.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && movement.Kind != model.DebtMovementPayment) {
			return ErrDebtPaymentNotFound
		}
		if err != nil {
			return err
		}

		if ledger, err = moveWallet(tx, userID, movement.WalletID, -movement.WalletDelta); err != nil {
			return err
		}
		if err := repo.DeleteMovement(movement.ID); err != nil {
			return err
		}
		debt.PaidAmount = roundMoney(debt.PaidAmount - movement.Amount)
		return repo.Update(debt)
	})
	if err != nil {
		return nil, err
	}

	s.alerter.BalanceChanges(userID, ledger.changes())
	return s.GetByID(userID, id)
}

// Delete membatalkan seluruh pergerakan saldo hutang (pokok dan pembayaran).
func (s *debtService) Delete(userID uuid.UUID, id uuid.UUID) error {
	var ledger *walletLedger

	err := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewDebtRepository(tx)
		if _, err := repo.FindByIDForUpdate(id, userID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDebtNotFound
			}
			return err
		}
		debt, err := repo.FindByID(id, userID)
		if err != nil {
			return err
		}

		var walletIDs []uuid.UUID
		for _, m := range debt.Movements {
			if m.WalletID != nil {
				walletIDs = append(walletIDs, *m.WalletID)
			}
		}
		if len(walletIDs) > 0 {
			if ledger, err = lockLedger(tx, userID, walletIDs...); err != nil {
				return err
			}
			if err := ledger.requireUsable(walletIDs...); err != nil {
				return err
			}
			for _, m := range debt.Movements {
				if m.WalletID != nil && m.WalletDelta < 0 {
					if err := ledger.apply(*m.WalletID, -m.WalletDelta); err != nil {
						return err
					}
				}
			}
			for _, m := range debt.Movements {
				if m.WalletID != nil && m.WalletDelta > 0 {
					if err := ledger.apply(*m.WalletID, -m.WalletDelta); err != nil {
						return err
					}
				}
			}
		}
		return repo.Delete(id)
	})
	if err != nil {
		return err
	}

	s.alerter.BalanceChanges(userID, ledger.changes())
	return nil
}

func (s *debtService) SendDueReminders(now time.Time) (int, error) {
	reminders, err := s.repo.ClaimReminders(today(now))
	if err != nil {
		return 0, err
	}
	for _, r := range reminders {
		when := "besok"
		if r.DueDate.Equal(today(now)) {
			when = "hari ini"
		}
		remaining := r.Principal - r.PaidAmount
		title, message := "Hutang Jatuh Tempo ⏰", fmt.Sprintf("Hutang ke %s sisa Rp %.0f jatuh tempo %s.", r.Counterparty, remaining, when)
		if r.Kind == model.DebtReceivable {
			title, message = "Tagih Piutang ⏰", fmt.Sprintf("Piutang dari %s sisa Rp %.0f jatuh tempo %s.", r.Counterparty, remaining, when)
		}
		_ = s.notif.CreateSystem(r.UserID, title, message)
	}
	return len(reminders), nil
}

func (s *debtService) toResponse(d *model.Debt, withMovements bool) *dto.DebtResponse {
	status := "open"
	if d.Remaining() < balanceEpsilon {
		status = "paid"
	}
	resp := &dto.DebtResponse{
		ID:           d.ID,
		Kind:         d.Kind,
		Counterparty: d.Counterparty,
		Principal:    d.Principal,
		PaidAmount:   d.PaidAmount,
		Remaining:    roundMoney(d.Remaining()),
		Status:       status,
		Note:         d.Note,
		DueDate:      formatDate(d.DueDate),
		Overdue:      status == "open" && d.DueDate != nil && d.DueDate.Before(today(s.now())),
		CreatedAt:    d.CreatedAt,
	}
	if withMovements {
		resp.Movements = make([]dto.DebtMovementResponse, 0, len(d.Movements))
		for _, m := range d.Movements {
			item := dto.DebtMovementResponse{
				ID:          m.ID,
				Kind:        m.Kind,
				Amount:      m.Amount,
				WalletDelta: m.WalletDelta,
				Date:        m.MovementDate.Format("2006-01-02"),
				Note:        m.Note,
			}
			if m.Wallet != nil {
				item.Wallet = &dto.TransferWallet{ID: m.Wallet.ID, Name: m.Wallet.Name}
			}
			resp.Movements = append(resp.Movements, item)
		}
	}
	return resp
}
