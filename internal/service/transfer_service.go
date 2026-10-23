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

const feeCategoryName = "Biaya Admin"

type TransferService interface {
	Create(userID uuid.UUID, req dto.CreateTransferRequest) (*dto.TransferResponse, error)
	GetByID(userID uuid.UUID, id uuid.UUID) (*dto.TransferResponse, error)
	GetAll(userID uuid.UUID, filter dto.TransferFilterRequest) ([]dto.TransferResponse, error)
	Delete(userID uuid.UUID, id uuid.UUID) error
}

type transferService struct {
	db           *gorm.DB
	transferRepo repository.TransferRepository
}

func NewTransferService(db *gorm.DB, transferRepo repository.TransferRepository) TransferService {
	return &transferService{db: db, transferRepo: transferRepo}
}

func feeCategory(tx *gorm.DB, userID uuid.UUID) (*model.Category, error) {
	var category model.Category
	err := tx.
		Where(model.Category{UserID: userID, Name: feeCategoryName, Type: "expense"}).
		FirstOrCreate(&category).Error
	return &category, err
}

func (s *transferService) Create(userID uuid.UUID, req dto.CreateTransferRequest) (*dto.TransferResponse, error) {
	if req.FromWalletID == req.ToWalletID {
		return nil, ErrTransferSameWallet
	}

	var createdID uuid.UUID

	err := s.db.Transaction(func(tx *gorm.DB) error {
		walletRepo := repository.NewWalletRepository(tx)

		wallets, err := lockWallets(walletRepo, userID, req.FromWalletID, req.ToWalletID)
		if err != nil {
			return err
		}
		from, to := wallets[req.FromWalletID], wallets[req.ToWalletID]
		if from.DeletedAt.Valid || to.DeletedAt.Valid {
			return ErrWalletNotFound
		}
		if from.Balance < req.Amount+req.Fee {
			return ErrInsufficientBalance
		}

		transfer := &model.Transfer{
			UserID:       userID,
			FromWalletID: req.FromWalletID,
			ToWalletID:   req.ToWalletID,
			Amount:       req.Amount,
			Fee:          req.Fee,
			Note:         req.Note,
			TransferDate: req.Date,
		}

		if req.Fee > 0 {
			category, err := feeCategory(tx, userID)
			if err != nil {
				return err
			}
			feeTx := &model.Transaction{
				UserID:          userID,
				WalletID:        req.FromWalletID,
				CategoryID:      category.ID,
				Amount:          req.Fee,
				Type:            "expense",
				Note:            fmt.Sprintf("Biaya admin transfer %s → %s", from.Name, to.Name),
				TransactionDate: req.Date,
			}
			if err := repository.NewTransactionRepository(tx).Create(feeTx); err != nil {
				return err
			}
			transfer.FeeTransactionID = &feeTx.ID
		}

		if err := repository.NewTransferRepository(tx).Create(transfer); err != nil {
			return err
		}
		createdID = transfer.ID

		if err := walletRepo.AddBalance(req.FromWalletID, userID, -(req.Amount + req.Fee)); err != nil {
			return err
		}
		return walletRepo.AddBalance(req.ToWalletID, userID, req.Amount)
	})
	if err != nil {
		return nil, err
	}

	return s.GetByID(userID, createdID)
}

func (s *transferService) GetByID(userID uuid.UUID, id uuid.UUID) (*dto.TransferResponse, error) {
	transfer, err := s.transferRepo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, err
	}
	return toTransferResponse(transfer), nil
}

func (s *transferService) GetAll(userID uuid.UUID, filter dto.TransferFilterRequest) ([]dto.TransferResponse, error) {
	transfers, err := s.transferRepo.FindAll(userID, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.TransferResponse, 0, len(transfers))
	for i := range transfers {
		responses = append(responses, *toTransferResponse(&transfers[i]))
	}
	return responses, nil
}

func (s *transferService) Delete(userID uuid.UUID, id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		transferRepo := repository.NewTransferRepository(tx)
		walletRepo := repository.NewWalletRepository(tx)

		transfer, err := transferRepo.FindByID(id, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTransferNotFound
		}
		if err != nil {
			return err
		}

		if _, err := lockWallets(walletRepo, userID, transfer.FromWalletID, transfer.ToWalletID); err != nil {
			return err
		}

		if err := transferRepo.Delete(id, userID); err != nil {
			return err
		}
		if transfer.FeeTransactionID != nil {
			if err := repository.NewTransactionRepository(tx).Delete(*transfer.FeeTransactionID, userID); err != nil {
				return err
			}
		}

		if err := walletRepo.AddBalance(transfer.FromWalletID, userID, transfer.Amount+transfer.Fee); err != nil {
			return err
		}
		return walletRepo.AddBalance(transfer.ToWalletID, userID, -transfer.Amount)
	})
}

func toTransferResponse(t *model.Transfer) *dto.TransferResponse {
	return &dto.TransferResponse{
		ID:               t.ID,
		FromWallet:       dto.TransferWallet{ID: t.FromWallet.ID, Name: t.FromWallet.Name},
		ToWallet:         dto.TransferWallet{ID: t.ToWallet.ID, Name: t.ToWallet.Name},
		Amount:           t.Amount,
		Fee:              t.Fee,
		FeeTransactionID: t.FeeTransactionID,
		Note:             t.Note,
		Date:             t.TransferDate,
		CreatedAt:        t.CreatedAt,
	}
}
