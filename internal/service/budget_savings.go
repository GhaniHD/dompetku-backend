package service

import (
	"fmt"
	"math"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/pkg/budgetcycle"

	"github.com/google/uuid"
)

// budgetLookup menyimpan data yang dipakai berulang saat menyusun daftar
// anggaran, agar saldo dompet dan nama dompet/target cukup diambil sekali.
type budgetLookup struct {
	s        *budgetService
	userID   uuid.UUID
	day      int
	balances map[string]map[uuid.UUID]float64
	wallets  map[uuid.UUID]model.Wallet
	goals    map[uuid.UUID]model.Goal
}

func (s *budgetService) newBudgetLookup(userID uuid.UUID, day int) *budgetLookup {
	return &budgetLookup{s: s, userID: userID, day: day, balances: map[string]map[uuid.UUID]float64{}}
}

func (l *budgetLookup) balancesBefore(date string) (map[uuid.UUID]float64, error) {
	if b, ok := l.balances[date]; ok {
		return b, nil
	}
	b, err := l.s.exportRepo.BalancesBefore(l.userID, date)
	if err != nil {
		return nil, err
	}
	l.balances[date] = b
	return b, nil
}

// deposited = perubahan saldo dompet selama periode: semua uang masuk dikurangi
// semua uang keluar, termasuk pengambilan untuk keperluan lain.
func (l *budgetLookup) deposited(year, month int, walletID uuid.UUID) (float64, error) {
	start, next := budgetcycle.Period(l.day, year, month)
	before, err := l.balancesBefore(start.Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	after, err := l.balancesBefore(next.Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	return after[walletID] - before[walletID], nil
}

func (l *budgetLookup) loadWallets() error {
	if l.wallets != nil {
		return nil
	}
	list, err := l.s.walletRepo.FindAll(l.userID, true)
	if err != nil {
		return err
	}
	l.wallets = make(map[uuid.UUID]model.Wallet, len(list))
	for _, w := range list {
		l.wallets[w.ID] = w
	}
	return nil
}

func (l *budgetLookup) loadGoals() error {
	if l.goals != nil {
		return nil
	}
	list, err := l.s.goalRepo.FindAll(l.userID)
	if err != nil {
		return err
	}
	l.goals = make(map[uuid.UUID]model.Goal, len(list))
	for _, g := range list {
		l.goals[g.ID] = g
	}
	return nil
}

func (l *budgetLookup) wallet(id uuid.UUID) (*dto.BudgetRef, error) {
	if err := l.loadWallets(); err != nil {
		return nil, err
	}
	w, ok := l.wallets[id]
	if !ok {
		return nil, fmt.Errorf("dompet %s untuk anggaran tabungan tidak ditemukan", id)
	}
	return &dto.BudgetRef{ID: w.ID, Name: w.Name}, nil
}

func (l *budgetLookup) goal(id uuid.UUID) (*dto.BudgetRef, error) {
	if err := l.loadGoals(); err != nil {
		return nil, err
	}
	g, ok := l.goals[id]
	if !ok {
		return nil, nil
	}
	return &dto.BudgetRef{ID: g.ID, Name: g.Name}, nil
}

// addFreeMoney menghitung sisa uang bebas untuk periode berjalan:
// saldo siap pakai − sisa batas belanja − sisa setoran tabungan.
func (s *budgetService) addFreeMoney(userID uuid.UUID, day int, summary *dto.BudgetSummaryResponse, lookup *budgetLookup) error {
	if y, m := budgetcycle.PeriodOf(day, s.now().In(jakarta)); y != summary.Year || m != summary.Month {
		return nil
	}
	if err := lookup.loadWallets(); err != nil {
		return err
	}
	if err := lookup.loadGoals(); err != nil {
		return err
	}

	reserved := map[uuid.UUID]bool{}
	for _, g := range lookup.goals {
		reserved[g.WalletID] = true
	}
	free := &dto.FreeMoneyResponse{}
	for _, b := range summary.Budgets {
		if b.Kind == model.BudgetSavings {
			reserved[b.Wallet.ID] = true
			free.SavingsLeft += b.Remain
			continue
		}
		free.BudgetLeft += math.Max(b.Remain, 0)
	}
	for _, w := range lookup.wallets {
		spendable := w.Type == model.WalletTypeCash || w.Type == model.WalletTypeBank || w.Type == model.WalletTypeEWallet
		if spendable && !w.IsArchived && !reserved[w.ID] && w.Balance > 0 {
			free.Spendable += w.Balance
		}
	}
	free.Amount = free.Spendable - free.BudgetLeft - free.SavingsLeft
	summary.FreeMoney = free
	return nil
}
