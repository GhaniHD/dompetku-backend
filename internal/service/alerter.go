package service

import (
	"context"
	"dompetku/internal/repository"
	"dompetku/pkg/budgetcycle"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	lowBalanceThreshold   = 50_000
	largeExpenseThreshold = 500_000
	largeIncomeThreshold  = 1_000_000
	budgetWarningPct      = 80
	budgetExceededPct     = 100
)

// Alerter membuat notifikasi sistem hanya saat sebuah ambang baru saja dilewati,
// sehingga user tidak menerima notifikasi yang sama berulang-ulang.
type Alerter struct {
	notif      NotificationService
	budgetRepo repository.BudgetRepository
	goalRepo   repository.GoalRepository
}

func NewAlerter(notif NotificationService, budgetRepo repository.BudgetRepository, goalRepo repository.GoalRepository) *Alerter {
	return &Alerter{notif: notif, budgetRepo: budgetRepo, goalRepo: goalRepo}
}

type budgetSnapshot struct {
	CategoryID  uuid.UUID
	Month       int
	Year        int
	Start       time.Time
	End         time.Time
	SpentBefore float64
}

// snapshotBudget mencatat pengeluaran kategori pada periode budget yang memuat
// tanggal transaksi (mengikuti tanggal mulai siklus user), sebelum perubahan.
// Dipanggil di dalam DB transaction sebelum transaksi expense dibuat atau diubah.
func snapshotBudget(tx *gorm.DB, userID, categoryID uuid.UUID, date time.Time) (*budgetSnapshot, error) {
	startDay, err := repository.NewUserRepository(tx).BudgetStartDay(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	startDay = budgetcycle.Normalize(startDay)

	year, month := budgetcycle.PeriodOf(startDay, date)
	start, end := budgetcycle.Period(startDay, year, month)

	spent, err := repository.NewBudgetRepository(tx).GetSpentByCategory(userID, categoryID, start, end)
	if err != nil {
		return nil, err
	}
	return &budgetSnapshot{CategoryID: categoryID, Month: month, Year: year, Start: start, End: end, SpentBefore: spent}, nil
}

func (a *Alerter) BalanceChanges(userID uuid.UUID, changes []balanceChange) {
	for _, c := range changes {
		if c.After > c.Before {
			achieved, err := a.goalRepo.MarkAchieved(c.Wallet.ID, c.After, time.Now())
			if err == nil {
				for _, g := range achieved {
					_ = a.notif.CreateSystem(userID,
						"Target Tabungan Tercapai 🎉",
						fmt.Sprintf("Target \"%s\" sebesar Rp %.0f sudah tercapai di dompet \"%s\".", g.Name, g.TargetAmount, c.Wallet.Name),
					)
				}
			}
		}
		if c.Wallet.AllowsNegative() {
			continue
		}
		if c.Before >= lowBalanceThreshold && c.After < lowBalanceThreshold {
			_ = a.notif.CreateSystem(userID,
				"Saldo Hampir Habis ⚠️",
				fmt.Sprintf("Saldo dompet \"%s\" tinggal Rp %.0f.", c.Wallet.Name, c.After),
			)
		}
	}
}

func (a *Alerter) BudgetUsage(userID uuid.UUID, snap *budgetSnapshot) {
	if snap == nil {
		return
	}
	budget, err := a.budgetRepo.FindByCategory(userID, snap.CategoryID, snap.Month, snap.Year)
	if err != nil || budget.Amount <= 0 {
		return
	}
	budget, err = a.budgetRepo.FindByID(budget.ID, userID)
	if err != nil {
		return
	}
	spent, err := a.budgetRepo.GetSpentByCategory(userID, snap.CategoryID, snap.Start, snap.End)
	if err != nil {
		return
	}

	before := snap.SpentBefore / budget.Amount * 100
	after := spent / budget.Amount * 100

	switch {
	case before < budgetExceededPct && after >= budgetExceededPct:
		_ = a.notif.CreateSystem(userID,
			"Anggaran Terlampaui ❗",
			fmt.Sprintf("Pengeluaran %s bulan %d/%d sudah Rp %.0f dari anggaran Rp %.0f (%.0f%%).",
				budget.Category.Name, snap.Month, snap.Year, spent, budget.Amount, after),
		)
	case before < budgetWarningPct && after >= budgetWarningPct:
		_ = a.notif.CreateSystem(userID,
			"Anggaran Hampir Habis ⚠️",
			fmt.Sprintf("Pengeluaran %s bulan %d/%d sudah %.0f%% dari anggaran (sisa Rp %.0f).",
				budget.Category.Name, snap.Month, snap.Year, after, budget.Amount-spent),
		)
	}
}

func (a *Alerter) LargeTransaction(userID uuid.UUID, txType string, amount float64, categoryName string) {
	switch {
	case txType == "expense" && amount >= largeExpenseThreshold:
		_ = a.notif.CreateSystem(userID,
			"Pengeluaran Besar Tercatat 💸",
			fmt.Sprintf("Pengeluaran Rp %.0f untuk kategori \"%s\" baru saja dicatat.", amount, categoryName),
		)
	case txType == "income" && amount >= largeIncomeThreshold:
		_ = a.notif.CreateSystem(userID,
			"Pemasukan Masuk 💰",
			fmt.Sprintf("Pemasukan Rp %.0f dari kategori \"%s\" berhasil dicatat.", amount, categoryName),
		)
	}
}
