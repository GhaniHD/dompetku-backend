package service

import (
	"math"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/pkg/budgetcycle"

	"github.com/google/uuid"
)

// addSavingsPlan menambahkan target tabungan sebagai baris anggaran, dan untuk
// periode berjalan menghitung sisa uang bebas.
func (s *budgetService) addSavingsPlan(userID uuid.UUID, day int, summary *dto.BudgetSummaryResponse) error {
	start, next := budgetcycle.Period(day, summary.Year, summary.Month)
	goals, err := s.goalRepo.FindAll(userID)
	if err != nil {
		return err
	}

	goalWallets := map[uuid.UUID]bool{}
	var active []model.Goal
	for _, g := range goals {
		goalWallets[g.WalletID] = true
		if !g.CreatedAt.Before(next) || (g.AchievedAt != nil && g.AchievedAt.Before(start)) {
			continue
		}
		active = append(active, g)
	}

	if len(active) > 0 {
		before, err := s.exportRepo.BalancesBefore(userID, start.Format("2006-01-02"))
		if err != nil {
			return err
		}
		after, err := s.exportRepo.BalancesBefore(userID, next.Format("2006-01-02"))
		if err != nil {
			return err
		}
		for _, g := range active {
			balanceStart := math.Max(before[g.WalletID], 0)
			item := dto.SavingsPlanResponse{
				GoalID: g.ID, Name: g.Name, WalletID: g.WalletID, WalletName: g.Wallet.Name,
				Deposited: after[g.WalletID] - before[g.WalletID],
				Deadline:  formatDate(g.Deadline),
				Achieved:  g.AchievedAt != nil,
			}
			if remaining := g.TargetAmount - balanceStart; remaining > 0 && g.Deadline != nil {
				months := max(monthsBetween(start, *g.Deadline), 1)
				item.Planned = math.Ceil(remaining / float64(months))
			}
			item.Remain = math.Max(item.Planned-item.Deposited, 0)
			summary.Savings = append(summary.Savings, item)
			summary.SavingsPlanned += item.Planned
			summary.SavingsDeposited += item.Deposited
		}
	}

	now := s.now().In(jakarta)
	if y, m := budgetcycle.PeriodOf(day, now); y != summary.Year || m != summary.Month {
		return nil
	}

	wallets, err := s.walletRepo.FindAll(userID, false)
	if err != nil {
		return err
	}
	free := &dto.FreeMoneyResponse{}
	for _, w := range wallets {
		spendable := w.Type == model.WalletTypeCash || w.Type == model.WalletTypeBank || w.Type == model.WalletTypeEWallet
		if spendable && !goalWallets[w.ID] && w.Balance > 0 {
			free.Spendable += w.Balance
		}
	}
	for _, b := range summary.Budgets {
		free.BudgetLeft += math.Max(b.Remain, 0)
	}
	for _, sv := range summary.Savings {
		free.SavingsLeft += sv.Remain
	}
	free.Amount = free.Spendable - free.BudgetLeft - free.SavingsLeft
	summary.FreeMoney = free
	return nil
}
