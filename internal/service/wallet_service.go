// internal/service/wallet_service.go
package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"

	"github.com/google/uuid"
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
	repo repository.WalletRepository
}

func NewWalletService(repo repository.WalletRepository) WalletService {
	return &walletService{repo: repo}
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (s *walletService) Create(userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
	wallet := &model.Wallet{
		UserID:  userID,
		Name:    req.Name,
		Balance: req.Balance,
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
	wallet, err := s.repo.FindByID(walletID, userID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		wallet.Name = *req.Name
	}
	if req.Balance != nil {
		wallet.Balance = *req.Balance
	}

	if err := s.repo.Update(wallet); err != nil {
		return nil, errors.New("gagal memperbarui dompet")
	}

	return toWalletResponse(wallet), nil
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