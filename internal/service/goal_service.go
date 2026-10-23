package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GoalService interface {
	Create(userID uuid.UUID, req dto.CreateGoalRequest) (*dto.GoalResponse, error)
	GetAll(userID uuid.UUID) ([]dto.GoalResponse, error)
	GetByID(userID uuid.UUID, id uuid.UUID) (*dto.GoalResponse, error)
	Update(userID uuid.UUID, id uuid.UUID, req dto.UpdateGoalRequest) (*dto.GoalResponse, error)
	Delete(userID uuid.UUID, id uuid.UUID) error
}

type goalService struct {
	repo       repository.GoalRepository
	walletRepo repository.WalletRepository
	now        func() time.Time
}

func NewGoalService(repo repository.GoalRepository, walletRepo repository.WalletRepository) GoalService {
	return &goalService{repo: repo, walletRepo: walletRepo, now: time.Now}
}

func (s *goalService) Create(userID uuid.UUID, req dto.CreateGoalRequest) (*dto.GoalResponse, error) {
	wallet, err := s.walletRepo.FindByID(req.WalletID, userID)
	if err != nil {
		return nil, err
	}
	if wallet.IsArchived {
		return nil, ErrWalletArchived
	}
	taken, err := s.repo.ExistsForWallet(req.WalletID, nil)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrGoalWalletTaken
	}

	goal := &model.Goal{
		UserID:       userID,
		WalletID:     req.WalletID,
		Name:         req.Name,
		TargetAmount: roundMoney(req.TargetAmount),
	}
	if req.Deadline != nil {
		d := dateOnlyUTC(*req.Deadline)
		goal.Deadline = &d
	}
	if wallet.Balance >= goal.TargetAmount {
		now := s.now()
		goal.AchievedAt = &now
	}
	if err := s.repo.Create(goal); err != nil {
		return nil, err
	}
	return s.GetByID(userID, goal.ID)
}

func (s *goalService) GetAll(userID uuid.UUID) ([]dto.GoalResponse, error) {
	goals, err := s.repo.FindAll(userID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.GoalResponse, 0, len(goals))
	for i := range goals {
		result = append(result, *s.toResponse(&goals[i]))
	}
	return result, nil
}

func (s *goalService) GetByID(userID uuid.UUID, id uuid.UUID) (*dto.GoalResponse, error) {
	goal, err := s.repo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGoalNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.toResponse(goal), nil
}

func (s *goalService) Update(userID uuid.UUID, id uuid.UUID, req dto.UpdateGoalRequest) (*dto.GoalResponse, error) {
	goal, err := s.repo.FindByID(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGoalNotFound
	}
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		goal.Name = *req.Name
	}
	if req.TargetAmount != nil {
		goal.TargetAmount = roundMoney(*req.TargetAmount)
		goal.AchievedAt = nil
		if goal.Wallet.Balance >= goal.TargetAmount {
			now := s.now()
			goal.AchievedAt = &now
		}
	}
	if req.Deadline != nil {
		d := dateOnlyUTC(*req.Deadline)
		goal.Deadline = &d
	}
	if err := s.repo.Update(goal); err != nil {
		return nil, err
	}
	return s.GetByID(userID, id)
}

func (s *goalService) Delete(userID uuid.UUID, id uuid.UUID) error {
	affected, err := s.repo.Delete(id, userID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrGoalNotFound
	}
	return nil
}

func monthsBetween(from, to time.Time) int {
	months := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		months--
	}
	return months
}

func (s *goalService) toResponse(g *model.Goal) *dto.GoalResponse {
	current := math.Max(g.Wallet.Balance, 0)
	remaining := math.Max(g.TargetAmount-current, 0)
	progress := math.Min(math.Round(current/g.TargetAmount*10000)/100, 100)

	resp := &dto.GoalResponse{
		ID:            g.ID,
		Name:          g.Name,
		Wallet:        dto.TransferWallet{ID: g.Wallet.ID, Name: g.Wallet.Name},
		TargetAmount:  g.TargetAmount,
		CurrentAmount: current,
		Remaining:     remaining,
		ProgressPct:   progress,
		Deadline:      formatDate(g.Deadline),
		Achieved:      g.AchievedAt != nil,
		AchievedAt:    g.AchievedAt,
	}
	if g.Deadline != nil {
		months := monthsBetween(today(s.now()), *g.Deadline)
		if months < 0 {
			months = 0
		}
		resp.MonthsLeft = &months
		if remaining > 0 {
			perMonth := math.Ceil(remaining / math.Max(float64(months), 1))
			resp.MonthlyNeeded = &perMonth
		}
	}
	return resp
}
