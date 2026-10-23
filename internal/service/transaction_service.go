package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"

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
	alerter         *Alerter
}

func NewTransactionService(
	db *gorm.DB,
	transactionRepo repository.TransactionRepository,
	alerter *Alerter,
) TransactionService {
	return &transactionService{
		db:              db,
		transactionRepo: transactionRepo,
		alerter:         alerter,
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

func (s *transactionService) CreateTransaction(userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionDetailResponse, error) {
	var (
		createdID uuid.UUID
		ledger    *walletLedger
		budget    *budgetSnapshot
	)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		ledger, err = lockLedger(tx, userID, req.WalletID)
		if err != nil {
			return err
		}
		if err := ledger.requireUsable(req.WalletID); err != nil {
			return err
		}
		if err := ensureCategory(tx, userID, req.CategoryID, req.Type); err != nil {
			return err
		}
		if req.Type == "expense" {
			if budget, err = snapshotBudget(tx, userID, req.CategoryID, req.Date); err != nil {
				return err
			}
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

		return ledger.apply(req.WalletID, deltaFromType(req.Type, req.Amount))
	})
	if err != nil {
		return nil, err
	}

	created, err := s.transactionRepo.FindByID(createdID, userID)
	if err != nil {
		return nil, err
	}

	s.alerter.BalanceChanges(userID, ledger.changes())
	s.alerter.BudgetUsage(userID, budget)
	s.alerter.LargeTransaction(userID, created.Type, created.Amount, created.Category.Name)
	return toDetailResponse(created), nil
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
	var (
		ledger *walletLedger
		budget *budgetSnapshot
	)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		transactionRepo := repository.NewTransactionRepository(tx)

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

		ledger, err = lockLedger(tx, userID, oldWalletID, t.WalletID)
		if err != nil {
			return err
		}
		if t.Type == "expense" {
			if budget, err = snapshotBudget(tx, userID, t.CategoryID, t.TransactionDate); err != nil {
				return err
			}
		}

		if err := transactionRepo.Update(t); err != nil {
			return err
		}

		newDelta := deltaFromType(t.Type, t.Amount)

		if oldWalletID == t.WalletID {
			diff := newDelta - oldDelta
			if diff == 0 {
				return nil
			}
			if err := ledger.requireUsable(t.WalletID); err != nil {
				return err
			}
			return ledger.apply(t.WalletID, diff)
		}

		if err := ledger.requireUsable(oldWalletID, t.WalletID); err != nil {
			return err
		}
		if err := ledger.apply(oldWalletID, -oldDelta); err != nil {
			return err
		}
		return ledger.apply(t.WalletID, newDelta)
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.transactionRepo.FindByID(id, userID)
	if err != nil {
		return nil, err
	}

	s.alerter.BalanceChanges(userID, ledger.changes())
	s.alerter.BudgetUsage(userID, budget)
	return toDetailResponse(updated), nil
}

func (s *transactionService) DeleteTransaction(userID uuid.UUID, id uuid.UUID) error {
	var ledger *walletLedger

	err := s.db.Transaction(func(tx *gorm.DB) error {
		transactionRepo := repository.NewTransactionRepository(tx)

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

		ledger, err = lockLedger(tx, userID, t.WalletID)
		if err != nil {
			return err
		}
		if err := ledger.requireUsable(t.WalletID); err != nil {
			return err
		}
		if err := transactionRepo.Delete(id, userID); err != nil {
			return err
		}

		return ledger.apply(t.WalletID, -deltaFromType(t.Type, t.Amount))
	})
	if err != nil {
		return err
	}

	s.alerter.BalanceChanges(userID, ledger.changes())
	return nil
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
