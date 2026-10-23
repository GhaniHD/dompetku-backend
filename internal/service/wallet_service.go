// internal/service/wallet_service.go
package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WalletService interface {
	Create(userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error)
	GetByID(userID uuid.UUID, walletID uuid.UUID) (*dto.WalletResponse, error)
	GetAll(userID uuid.UUID) ([]dto.WalletResponse, error)
	Update(userID uuid.UUID, walletID uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error)
	Delete(userID uuid.UUID, walletID uuid.UUID) error
	GetTotalBalance(userID uuid.UUID) (float64, error)
}

type walletService struct {
	db   *gorm.DB
	repo repository.WalletRepository
}

func NewWalletService(db *gorm.DB, repo repository.WalletRepository) WalletService {
	return &walletService{db: db, repo: repo}
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (s *walletService) Create(userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
	wallet := &model.Wallet{
		UserID:         userID,
		Name:           req.Name,
		Balance:        req.Balance,
		OpeningBalance: req.Balance,
	}

	if err := s.repo.Create(wallet); err != nil {
		return nil, errors.New("gagal membuat dompet")
	}

	return toWalletResponse(wallet), nil
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func (s *walletService) GetByID(userID uuid.UUID, walletID uuid.UUID) (*dto.WalletResponse, error) {
	wallet, err := s.repo.FindByID(walletID, userID)
	if err != nil {
		return nil, err
	}
	return toWalletResponse(wallet), nil
}

// ─── GetAll ───────────────────────────────────────────────────────────────────

func (s *walletService) GetAll(userID uuid.UUID) ([]dto.WalletResponse, error) {
	wallets, err := s.repo.FindAll(userID)
	if err != nil {
		return nil, errors.New("gagal mengambil data dompet")
	}

	result := make([]dto.WalletResponse, len(wallets))
	for i, w := range wallets {
		result[i] = *toWalletResponse(&w)
	}
	return result, nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (s *walletService) Update(userID uuid.UUID, walletID uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error) {
	var updated *model.Wallet

	err := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewWalletRepository(tx)

		wallet, err := repo.FindByIDForUpdate(walletID, userID)
		if err != nil {
			return err
		}
		if wallet.DeletedAt.Valid {
			return repository.ErrWalletNotFound
		}

		if req.Name != nil {
			wallet.Name = *req.Name
		}
		// Edit saldo langsung digeser ke opening_balance supaya invarian saldo tetap
		// terjaga sampai fitur penyesuaian saldo tersedia.
		if req.Balance != nil {
			wallet.OpeningBalance += *req.Balance - wallet.Balance
			wallet.Balance = *req.Balance
		}

		if err := repo.Update(wallet); err != nil {
			return err
		}
		updated = wallet
		return nil
	})
	if errors.Is(err, repository.ErrWalletNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, errors.New("gagal memperbarui dompet")
	}

	return toWalletResponse(updated), nil
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (s *walletService) Delete(userID uuid.UUID, walletID uuid.UUID) error {
	return s.repo.Delete(walletID, userID)
}

// ─── GetTotalBalance ──────────────────────────────────────────────────────────

func (s *walletService) GetTotalBalance(userID uuid.UUID) (float64, error) {
	total, err := s.repo.SumBalance(userID)
	if err != nil {
		return 0, errors.New("gagal menghitung total saldo")
	}
	return total, nil
}

// ─── Helper mapper ────────────────────────────────────────────────────────────

func toWalletResponse(w *model.Wallet) *dto.WalletResponse {
	return &dto.WalletResponse{
		ID:        w.ID,
		UserID:    w.UserID,
		Name:      w.Name,
		Balance:   w.Balance,
		CreatedAt: w.CreatedAt,
	}
}
