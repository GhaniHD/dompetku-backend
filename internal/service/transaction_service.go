package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"
	"fmt"
	"sort"

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
	db              *gorm.DB
	transactionRepo repository.TransactionRepository
	walletRepo      repository.WalletRepository
	notifSvc        NotificationService
}

func NewTransactionService(
	db *gorm.DB,
	transactionRepo repository.TransactionRepository,
	walletRepo repository.WalletRepository,
	notifSvc NotificationService,
) TransactionService {
	return &transactionService{
		db:              db,
		transactionRepo: transactionRepo,
		walletRepo:      walletRepo,
		notifSvc:        notifSvc,
	}
}

func deltaFromType(txType string, amount float64) float64 {
	if txType == "income" {
		return amount
	}
	return -amount
}

func ensureCategory(tx *gorm.DB, userID, categoryID uuid.UUID, txType string) error {
	category, err := repository.NewCategoryRepository(tx).FindByID(categoryID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCategoryNotFound
	}
	if err != nil {
		return err
	}
	if category.Type != txType {
		return ErrCategoryTypeMismatch
	}
	return nil
}

func ensureNotTransferFee(tx *gorm.DB, transactionID uuid.UUID) error {
	isFee, err := repository.NewTransferRepository(tx).IsFeeTransaction(transactionID)
	if err != nil {
		return err
	}
	if isFee {
		return ErrTransactionIsTransferFee
	}
	return nil
}

// lockWallets mengunci wallet selalu dalam urutan ID yang sama agar dua request
// yang menyentuh wallet yang sama tidak saling deadlock.
func lockWallets(repo repository.WalletRepository, userID uuid.UUID, ids ...uuid.UUID) (map[uuid.UUID]*model.Wallet, error) {
	unique := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i].String() < unique[j].String() })

	locked := make(map[uuid.UUID]*model.Wallet, len(unique))
	for _, id := range unique {
		wallet, err := repo.FindByIDForUpdate(id, userID)
		if err != nil {
			return nil, err
		}
		locked[id] = wallet
	}
	return locked, nil
}

func (s *transactionService) CreateTransaction(userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionDetailResponse, error) {
	var createdID uuid.UUID

	err := s.db.Transaction(func(tx *gorm.DB) error {
		walletRepo := repository.NewWalletRepository(tx)

		wallets, err := lockWallets(walletRepo, userID, req.WalletID)
		if err != nil {
			return err
		}
		if wallets[req.WalletID].DeletedAt.Valid {
			return ErrWalletNotFound
		}
		if err := ensureCategory(tx, userID, req.CategoryID, req.Type); err != nil {
			return err
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
		if err := repository.NewTransactionRepository(tx).Create(transaction); err != nil {
			return err
		}
		createdID = transaction.ID

		return walletRepo.AddBalance(req.WalletID, userID, deltaFromType(req.Type, req.Amount))
	})
	if err != nil {
		return nil, err
	}

	created, err := s.transactionRepo.FindByID(createdID, userID)
	if err != nil {
		return nil, err
	}

	s.notifyAfterCreate(userID, created)
	return toDetailResponse(created), nil
}

func (s *transactionService) notifyAfterCreate(userID uuid.UUID, t *model.Transaction) {
	wallet, err := s.walletRepo.FindByID(t.WalletID, userID)
	if err == nil {
		if wallet.Balance < 50_000 && wallet.Balance >= 0 {
			_ = s.notifSvc.CreateSystem(userID,
				"Saldo Hampir Habis ⚠️",
				fmt.Sprintf("Saldo dompet \"%s\" tinggal Rp %.0f. Segera isi ulang.", wallet.Name, wallet.Balance),
			)
		}
		if wallet.Balance < 0 {
			_ = s.notifSvc.CreateSystem(userID,
				"Saldo Minus ❗",
				fmt.Sprintf("Saldo dompet \"%s\" minus Rp %.0f setelah transaksi terakhir.", wallet.Name, -wallet.Balance),
			)
		}
	}

	if t.Type == "expense" && t.Amount >= 500_000 {
		_ = s.notifSvc.CreateSystem(userID,
			"Pengeluaran Besar Tercatat 💸",
			fmt.Sprintf("Pengeluaran sebesar Rp %.0f untuk kategori \"%s\" baru saja dicatat.", t.Amount, t.Category.Name),
		)
	}

	if t.Type == "income" && t.Amount >= 1_000_000 {
		_ = s.notifSvc.CreateSystem(userID,
			"Pemasukan Masuk 💰",
			fmt.Sprintf("Pemasukan sebesar Rp %.0f dari kategori \"%s\" berhasil dicatat.", t.Amount, t.Category.Name),
		)
	}
}

func (s *transactionService) GetTransactionByID(userID uuid.UUID, id uuid.UUID) (*dto.TransactionDetailResponse, error) {
	transaction, err := s.transactionRepo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTransactionNotFound
	}
	if err != nil {
		return nil, err
	}
	return toDetailResponse(transaction), nil
}

func (s *transactionService) GetAllTransactions(userID uuid.UUID, filter dto.TransactionFilterRequest) ([]dto.TransactionResponse, error) {
	transactions, err := s.transactionRepo.FindAll(userID, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.TransactionResponse, 0, len(transactions))
	for _, t := range transactions {
		responses = append(responses, toListResponse(t))
	}
	return responses, nil
}

func (s *transactionService) UpdateTransaction(userID uuid.UUID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionDetailResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		transactionRepo := repository.NewTransactionRepository(tx)
		walletRepo := repository.NewWalletRepository(tx)

		t, err := transactionRepo.FindByID(id, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTransactionNotFound
		}
		if err != nil {
			return err
		}
		if err := ensureNotTransferFee(tx, id); err != nil {
			return err
		}

		oldWalletID := t.WalletID
		oldDelta := deltaFromType(t.Type, t.Amount)
		needCategoryCheck := false

		if req.WalletID != nil {
			t.WalletID = *req.WalletID
		}
		if req.CategoryID != nil && *req.CategoryID != t.CategoryID {
			t.CategoryID = *req.CategoryID
			needCategoryCheck = true
		}
		if req.Type != nil && *req.Type != t.Type {
			t.Type = *req.Type
			needCategoryCheck = true
		}
		if req.Amount != nil {
			t.Amount = *req.Amount
		}
		if req.Note != nil {
			t.Note = *req.Note
		}
		if req.Date != nil {
			t.TransactionDate = *req.Date
		}

		if needCategoryCheck {
			if err := ensureCategory(tx, userID, t.CategoryID, t.Type); err != nil {
				return err
			}
		}

		wallets, err := lockWallets(walletRepo, userID, oldWalletID, t.WalletID)
		if err != nil {
			return err
		}
		if t.WalletID != oldWalletID && wallets[t.WalletID].DeletedAt.Valid {
			return ErrWalletNotFound
		}

		if err := transactionRepo.Update(t); err != nil {
			return err
		}

		newDelta := deltaFromType(t.Type, t.Amount)

		if oldWalletID == t.WalletID {
			if diff := newDelta - oldDelta; diff != 0 {
				return walletRepo.AddBalance(t.WalletID, userID, diff)
			}
			return nil
		}

		if err := walletRepo.AddBalance(oldWalletID, userID, -oldDelta); err != nil {
			return err
		}
		return walletRepo.AddBalance(t.WalletID, userID, newDelta)
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.transactionRepo.FindByID(id, userID)
	if err != nil {
		return nil, err
	}
	return toDetailResponse(updated), nil
}

func (s *transactionService) DeleteTransaction(userID uuid.UUID, id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		transactionRepo := repository.NewTransactionRepository(tx)
		walletRepo := repository.NewWalletRepository(tx)

		t, err := transactionRepo.FindByID(id, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTransactionNotFound
		}
		if err != nil {
			return err
		}
		if err := ensureNotTransferFee(tx, id); err != nil {
			return err
		}

		if _, err := lockWallets(walletRepo, userID, t.WalletID); err != nil {
			return err
		}
		if err := transactionRepo.Delete(id, userID); err != nil {
			return err
		}

		return walletRepo.AddBalance(t.WalletID, userID, -deltaFromType(t.Type, t.Amount))
	})
}

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
