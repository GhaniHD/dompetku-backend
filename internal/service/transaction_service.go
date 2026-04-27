// internal/service/transaction_service.go
package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TransactionService interface {
	CreateTransaction(userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionDetailResponse, error)
	GetTransactionByID(userID uuid.UUID, id uuid.UUID) (*dto.TransactionDetailResponse, error)
	GetAllTransactions(userID uuid.UUID, filter dto.TransactionFilterRequest) ([]dto.TransactionResponse, error)
	UpdateTransaction(userID uuid.UUID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionDetailResponse, error)
	DeleteTransaction(userID uuid.UUID, id uuid.UUID) error
}

type transactionService struct {
	transactionRepo repository.TransactionRepository
	walletRepo      repository.WalletRepository
	notifSvc        NotificationService // ← tambah
}

func NewTransactionService(
	transactionRepo repository.TransactionRepository,
	walletRepo      repository.WalletRepository,
	notifSvc        NotificationService, // ← tambah
) TransactionService {
	return &transactionService{
		transactionRepo: transactionRepo,
		walletRepo:      walletRepo,
		notifSvc:        notifSvc,
	}
}

// ─── deltaFromType ────────────────────────────────────────────────────────────

func deltaFromType(txType string, amount float64) float64 {
	if txType == "income" {
		return amount
	}
	return -amount
}

// ─── CreateTransaction ────────────────────────────────────────────────────────

func (s *transactionService) CreateTransaction(userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionDetailResponse, error) {
	if req.Type != "income" && req.Type != "expense" {
		return nil, errors.New("tipe transaksi harus 'income' atau 'expense'")
	}

	transaction := &model.Transaction{
		UserID:          userID,
		WalletID:        req.WalletID,
		CategoryID:      req.CategoryID,
		Amount:          req.Amount,
		Type:            req.Type,
		Note:            req.Note,
		TransactionDate: req.Date,
	}

	if err := s.transactionRepo.Create(transaction); err != nil {
		return nil, err
	}

	delta := deltaFromType(req.Type, req.Amount)
	if err := s.walletRepo.AddBalance(req.WalletID, userID, delta); err != nil {
		return nil, errors.New("transaksi tersimpan tapi gagal update saldo dompet")
	}

	created, err := s.transactionRepo.FindByID(transaction.ID, userID)
	if err != nil {
		return nil, err
	}

	// ── Cek saldo dompet setelah transaksi ────────────────────────────
	wallet, err := s.walletRepo.FindByID(req.WalletID, userID)
	if err == nil {
		// Saldo hampir habis: di bawah 50.000
		if wallet.Balance < 50_000 && wallet.Balance >= 0 {
			_ = s.notifSvc.CreateSystem(userID,
				"Saldo Hampir Habis ⚠️",
				fmt.Sprintf("Saldo dompet \"%s\" tinggal Rp %.0f. Segera isi ulang.", wallet.Name, wallet.Balance),
			)
		}
		// Saldo minus
		if wallet.Balance < 0 {
			_ = s.notifSvc.CreateSystem(userID,
				"Saldo Minus ❗",
				fmt.Sprintf("Saldo dompet \"%s\" minus Rp %.0f setelah transaksi terakhir.", wallet.Name, -wallet.Balance),
			)
		}
	}

	// ── Pengeluaran besar (≥ 500.000) ─────────────────────────────────
	if req.Type == "expense" && req.Amount >= 500_000 {
		_ = s.notifSvc.CreateSystem(userID,
			"Pengeluaran Besar Tercatat 💸",
			fmt.Sprintf("Pengeluaran sebesar Rp %.0f untuk kategori \"%s\" baru saja dicatat.",
				req.Amount, created.Category.Name),
		)
	}

	// ── Pemasukan besar (≥ 1.000.000) ─────────────────────────────────
	if req.Type == "income" && req.Amount >= 1_000_000 {
		_ = s.notifSvc.CreateSystem(userID,
			"Pemasukan Masuk 💰",
			fmt.Sprintf("Pemasukan sebesar Rp %.0f dari kategori \"%s\" berhasil dicatat.",
				req.Amount, created.Category.Name),
		)
	}

	return toDetailResponse(created), nil
}

// ─── GetTransactionByID ───────────────────────────────────────────────────────

func (s *transactionService) GetTransactionByID(userID uuid.UUID, id uuid.UUID) (*dto.TransactionDetailResponse, error) {
	transaction, err := s.transactionRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("transaksi tidak ditemukan")
		}
		return nil, err
	}
	return toDetailResponse(transaction), nil
}

// ─── GetAllTransactions ───────────────────────────────────────────────────────

func (s *transactionService) GetAllTransactions(userID uuid.UUID, filter dto.TransactionFilterRequest) ([]dto.TransactionResponse, error) {
	transactions, err := s.transactionRepo.FindAll(userID, filter)
	if err != nil {
		return nil, err
	}

	var responses []dto.TransactionResponse
	for _, t := range transactions {
		responses = append(responses, toListResponse(t))
	}
	if responses == nil {
		responses = []dto.TransactionResponse{}
	}
	return responses, nil
}

// ─── UpdateTransaction ────────────────────────────────────────────────────────

func (s *transactionService) UpdateTransaction(userID uuid.UUID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionDetailResponse, error) {
	old, err := s.transactionRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("transaksi tidak ditemukan")
		}
		return nil, err
	}

	if req.Type != "" && req.Type != "income" && req.Type != "expense" {
		return nil, errors.New("tipe transaksi harus 'income' atau 'expense'")
	}

	oldWalletID := old.WalletID
	oldDelta := deltaFromType(old.Type, old.Amount)

	if req.WalletID != uuid.Nil {
		old.WalletID = req.WalletID
	}
	if req.CategoryID != uuid.Nil {
		old.CategoryID = req.CategoryID
	}
	if req.Amount != 0 {
		old.Amount = req.Amount
	}
	if req.Type != "" {
		old.Type = req.Type
	}
	if req.Note != "" {
		old.Note = req.Note
	}
	if !req.Date.IsZero() {
		old.TransactionDate = req.Date
	}

	if err := s.transactionRepo.Update(old); err != nil {
		return nil, err
	}

	newDelta := deltaFromType(old.Type, old.Amount)

	if oldWalletID == old.WalletID {
		diff := newDelta - oldDelta
		if diff != 0 {
			if err := s.walletRepo.AddBalance(old.WalletID, userID, diff); err != nil {
				return nil, errors.New("transaksi diperbarui tapi gagal update saldo dompet")
			}
		}
	} else {
		if err := s.walletRepo.AddBalance(oldWalletID, userID, -oldDelta); err != nil {
			return nil, errors.New("gagal mengembalikan saldo dompet lama")
		}
		if err := s.walletRepo.AddBalance(old.WalletID, userID, newDelta); err != nil {
			return nil, errors.New("gagal menambahkan saldo dompet baru")
		}
	}

	updated, err := s.transactionRepo.FindByID(old.ID, userID)
	if err != nil {
		return nil, err
	}
	return toDetailResponse(updated), nil
}

// ─── DeleteTransaction ────────────────────────────────────────────────────────

func (s *transactionService) DeleteTransaction(userID uuid.UUID, id uuid.UUID) error {
	tx, err := s.transactionRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("transaksi tidak ditemukan")
		}
		return err
	}

	if err := s.transactionRepo.Delete(id, userID); err != nil {
		return err
	}

	delta := deltaFromType(tx.Type, tx.Amount)
	if err := s.walletRepo.AddBalance(tx.WalletID, userID, -delta); err != nil {
		return errors.New("transaksi dihapus tapi gagal mengembalikan saldo dompet")
	}

	return nil
}

// ─── Helper mapper ────────────────────────────────────────────────────────────

func toListResponse(t model.Transaction) dto.TransactionResponse {
	return dto.TransactionResponse{
		ID:           t.ID,
		Amount:       t.Amount,
		Type:         t.Type,
		Note:         t.Note,
		Date:         t.TransactionDate,
		WalletName:   t.Wallet.Name,
		CategoryName: t.Category.Name,
	}
}

func toDetailResponse(t *model.Transaction) *dto.TransactionDetailResponse {
	resp := &dto.TransactionDetailResponse{
		ID:     t.ID,
		Amount: t.Amount,
		Type:   t.Type,
		Note:   t.Note,
		Date:   t.TransactionDate,
	}
	resp.Wallet.ID = t.Wallet.ID
	resp.Wallet.Name = t.Wallet.Name
	resp.Category.ID = t.Category.ID
	resp.Category.Name = t.Category.Name
	return resp
}