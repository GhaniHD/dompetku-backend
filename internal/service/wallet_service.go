package service

import (
	"database/sql"
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WalletService interface {
	Create(userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error)
	GetByID(userID uuid.UUID, walletID uuid.UUID) (*dto.WalletResponse, error)
	GetAll(userID uuid.UUID, filter dto.WalletFilterRequest) ([]dto.WalletResponse, error)
	Update(userID uuid.UUID, walletID uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error)
	// Delete mengarsipkan wallet yang punya riwayat, dan menghapus wallet yang belum pernah dipakai.
	Delete(userID uuid.UUID, walletID uuid.UUID) (archived bool, err error)
	GetTotalBalance(userID uuid.UUID) (float64, error)
	Adjust(userID uuid.UUID, walletID uuid.UUID, req dto.AdjustBalanceRequest) (*dto.BalanceAdjustmentResponse, error)
	ListAdjustments(userID uuid.UUID, walletID uuid.UUID) ([]dto.BalanceAdjustmentResponse, error)
	History(userID uuid.UUID, walletID uuid.UUID, filter dto.WalletHistoryFilterRequest) (*dto.WalletHistoryResponse, error)
	// OpeningToIncome memindahkan saldo awal menjadi transaksi pemasukan tanpa
	// mengubah saldo, sehingga ikut terhitung di laporan dan ringkasan.
	OpeningToIncome(userID uuid.UUID, walletID uuid.UUID, req dto.OpeningIncomeRequest) (*dto.WalletResponse, error)
}

type walletService struct {
	db             *gorm.DB
	repo           repository.WalletRepository
	adjustmentRepo repository.AdjustmentRepository
	historyRepo    repository.HistoryRepository
}

func NewWalletService(
	db *gorm.DB,
	repo repository.WalletRepository,
	adjustmentRepo repository.AdjustmentRepository,
	historyRepo repository.HistoryRepository,
) WalletService {
	return &walletService{db: db, repo: repo, adjustmentRepo: adjustmentRepo, historyRepo: historyRepo}
}

const defaultHistoryLimit = 20

func (s *walletService) History(userID uuid.UUID, walletID uuid.UUID, filter dto.WalletHistoryFilterRequest) (*dto.WalletHistoryResponse, error) {
	wallet, err := s.repo.FindByID(walletID, userID)
	if err != nil {
		return nil, err
	}

	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultHistoryLimit
	}

	entries, total, err := s.historyRepo.WalletHistory(walletID, wallet.OpeningBalance, filter, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}

	return &dto.WalletHistoryResponse{
		Wallet:         *toWalletResponse(wallet),
		OpeningBalance: wallet.OpeningBalance,
		Entries:        entries,
		Total:          total,
		Page:           page,
		Limit:          limit,
	}, nil
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

func (s *walletService) Create(userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
	walletType := req.Type
	if walletType == "" {
		walletType = model.WalletTypeCash
	}

	wallet := &model.Wallet{
		UserID:         userID,
		Name:           req.Name,
		Type:           walletType,
		Balance:        roundMoney(req.Balance),
		OpeningBalance: roundMoney(req.Balance),
	}
	if wallet.Balance < 0 && !wallet.AllowsNegative() {
		return nil, ErrNegativeOpeningBalance
	}

	if err := s.repo.Create(wallet); err != nil {
		return nil, err
	}
	return toWalletResponse(wallet), nil
}

func (s *walletService) GetByID(userID uuid.UUID, walletID uuid.UUID) (*dto.WalletResponse, error) {
	wallet, err := s.repo.FindByID(walletID, userID)
	if err != nil {
		return nil, err
	}
	return toWalletResponse(wallet), nil
}

func (s *walletService) GetAll(userID uuid.UUID, filter dto.WalletFilterRequest) ([]dto.WalletResponse, error) {
	wallets, err := s.repo.FindAll(userID, filter.IncludeArchived)
	if err != nil {
		return nil, err
	}

	result := make([]dto.WalletResponse, len(wallets))
	for i := range wallets {
		result[i] = *toWalletResponse(&wallets[i])
	}
	return result, nil
}

func (s *walletService) Update(userID uuid.UUID, walletID uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error) {
	var updated *model.Wallet

	err := s.db.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockLedger(tx, userID, walletID)
		if err != nil {
			return err
		}
		wallet := ledger.wallet(walletID)
		if wallet.DeletedAt.Valid {
			return ErrWalletNotFound
		}

		if req.IsArchived != nil && !*req.IsArchived {
			wallet.IsArchived = false
		}
		if req.Name != nil {
			wallet.Name = *req.Name
		}
		if req.Balance != nil && math.Abs(*req.Balance-wallet.Balance) >= balanceEpsilon {
			if _, err := recordAdjustment(tx, ledger, walletID, *req.Balance, "Penyesuaian dari edit dompet", time.Now()); err != nil {
				return err
			}
		}
		if req.Type != nil {
			wallet.Type = *req.Type
			if wallet.Balance < 0 && !wallet.AllowsNegative() {
				return ErrWalletTypeNegative
			}
		}
		if req.IsArchived != nil && *req.IsArchived {
			if math.Abs(wallet.Balance) >= balanceEpsilon {
				return ErrWalletNotEmpty
			}
			wallet.IsArchived = true
		}

		if err := ledger.repo.Update(wallet); err != nil {
			return err
		}
		updated = wallet
		return nil
	})
	if err != nil {
		return nil, err
	}

	return toWalletResponse(updated), nil
}

func (s *walletService) Delete(userID uuid.UUID, walletID uuid.UUID) (bool, error) {
	archived := false

	err := s.db.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockLedger(tx, userID, walletID)
		if err != nil {
			return err
		}
		wallet := ledger.wallet(walletID)
		if wallet.DeletedAt.Valid {
			return ErrWalletNotFound
		}

		hasHistory, err := ledger.repo.HasHistory(walletID)
		if err != nil {
			return err
		}
		if !hasHistory {
			return ledger.repo.Delete(walletID, userID)
		}

		if math.Abs(wallet.Balance) >= balanceEpsilon {
			return ErrWalletNotEmpty
		}
		wallet.IsArchived = true
		archived = true
		return ledger.repo.Update(wallet)
	})
	return archived, err
}

func (s *walletService) GetTotalBalance(userID uuid.UUID) (float64, error) {
	return s.repo.SumBalance(userID)
}

func (s *walletService) Adjust(userID uuid.UUID, walletID uuid.UUID, req dto.AdjustBalanceRequest) (*dto.BalanceAdjustmentResponse, error) {
	date := time.Now()
	if req.Date != nil {
		date = *req.Date
	}

	var created *model.BalanceAdjustment
	err := s.db.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockLedger(tx, userID, walletID)
		if err != nil {
			return err
		}
		created, err = recordAdjustment(tx, ledger, walletID, *req.ActualBalance, req.Note, date)
		return err
	})
	if err != nil {
		return nil, err
	}
	return toAdjustmentResponse(created), nil
}

func (s *walletService) ListAdjustments(userID uuid.UUID, walletID uuid.UUID) ([]dto.BalanceAdjustmentResponse, error) {
	if _, err := s.repo.FindByID(walletID, userID); err != nil {
		return nil, err
	}

	adjustments, err := s.adjustmentRepo.FindByWallet(walletID, userID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.BalanceAdjustmentResponse, len(adjustments))
	for i := range adjustments {
		result[i] = *toAdjustmentResponse(&adjustments[i])
	}
	return result, nil
}

// recordAdjustment mencatat selisih saldo sebagai penyesuaian sehingga riwayat
// tetap bisa menjelaskan saldo akhir. Wallet harus sudah dikunci lewat ledger.
func recordAdjustment(tx *gorm.DB, ledger *walletLedger, walletID uuid.UUID, actual float64, note string, date time.Time) (*model.BalanceAdjustment, error) {
	if err := ledger.requireUsable(walletID); err != nil {
		return nil, err
	}

	wallet := ledger.wallet(walletID)
	actual = roundMoney(actual)
	delta := roundMoney(actual - wallet.Balance)
	if math.Abs(delta) < balanceEpsilon {
		return nil, ErrAdjustmentNoChange
	}

	previous := wallet.Balance
	if err := ledger.apply(walletID, delta); err != nil {
		return nil, err
	}

	adjustment := &model.BalanceAdjustment{
		UserID:          ledger.userID,
		WalletID:        walletID,
		Amount:          delta,
		PreviousBalance: previous,
		NewBalance:      actual,
		Note:            note,
		AdjustedAt:      date,
	}
	if err := repository.NewAdjustmentRepository(tx).Create(adjustment); err != nil {
		return nil, err
	}
	return adjustment, nil
}

func toWalletResponse(w *model.Wallet) *dto.WalletResponse {
	return &dto.WalletResponse{
		ID:         w.ID,
		UserID:     w.UserID,
		Name:       w.Name,
		Type:       w.Type,
		Balance:    w.Balance,
		IsArchived: w.IsArchived,
		CreatedAt:  w.CreatedAt,
	}
}

func toAdjustmentResponse(a *model.BalanceAdjustment) *dto.BalanceAdjustmentResponse {
	return &dto.BalanceAdjustmentResponse{
		ID:              a.ID,
		WalletID:        a.WalletID,
		Amount:          a.Amount,
		PreviousBalance: a.PreviousBalance,
		NewBalance:      a.NewBalance,
		Note:            a.Note,
		Date:            a.AdjustedAt,
		CreatedAt:       a.CreatedAt,
	}
}

func (s *walletService) OpeningToIncome(userID uuid.UUID, walletID uuid.UUID, req dto.OpeningIncomeRequest) (*dto.WalletResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewWalletRepository(tx)
		wallet, err := repo.FindByIDForUpdate(walletID, userID)
		if err != nil {
			return err
		}
		if wallet.DeletedAt.Valid {
			return ErrWalletNotFound
		}
		amount := wallet.OpeningBalance
		if amount <= 0 {
			return ErrNoOpeningBalance
		}

		// Tanggal pemasukan = saat dompet dibuat atau mutasi pertamanya, mana yang lebih
		// awal, agar saldo berjalan di riwayat tidak pernah tampil minus.
		date := wallet.CreatedAt.In(jakarta)
		date = time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, jakarta)
		var first sql.NullTime
		if err := tx.Raw(`SELECT MIN(d) FROM (
				SELECT MIN(transaction_date) AS d FROM transactions WHERE wallet_id = ? AND deleted_at IS NULL
				UNION ALL SELECT MIN(transfer_date) FROM transfers WHERE (from_wallet_id = ? OR to_wallet_id = ?) AND deleted_at IS NULL
				UNION ALL SELECT MIN(adjusted_at) FROM balance_adjustments WHERE wallet_id = ? AND deleted_at IS NULL
				UNION ALL SELECT MIN(movement_date) FROM debt_movements WHERE wallet_id = ? AND deleted_at IS NULL
			) x`, walletID, walletID, walletID, walletID, walletID).Scan(&first).Error; err != nil {
			return err
		}
		if first.Valid {
			f := time.Date(first.Time.Year(), first.Time.Month(), first.Time.Day(), 12, 0, 0, 0, jakarta)
			if f.Before(date) {
				date = f
			}
		}

		note := strings.TrimSpace(req.Note)
		if note == "" {
			note = "Saldo awal " + wallet.Name
		}
		created, err := createTransactionTx(tx, userID, dto.CreateTransactionRequest{
			WalletID: walletID, CategoryID: req.CategoryID, Amount: amount, Type: "income", Note: note, Date: date,
		}, nil)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.Transaction{}).Where("id = ?", created.ID).
			UpdateColumn("created_at", wallet.CreatedAt).Error; err != nil {
			return err
		}
		return tx.Model(&model.Wallet{}).
			Where("id = ? AND user_id = ?", walletID, userID).
			Updates(map[string]any{
				"opening_balance": gorm.Expr("opening_balance - ?", amount),
				"balance":         gorm.Expr("balance - ?", amount),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(userID, walletID)
}
