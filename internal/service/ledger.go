package service

import (
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"sort"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Toleransi pembulatan float64 saat membandingkan saldo dengan nol.
const balanceEpsilon = 0.005

// walletLedger memegang wallet yang sudah dikunci di dalam satu DB transaction.
// Semua perubahan saldo wajib lewat apply agar aturan saldo minus konsisten.
type walletLedger struct {
	repo    repository.WalletRepository
	userID  uuid.UUID
	wallets map[uuid.UUID]*model.Wallet
}

// lockLedger mengunci wallet selalu dalam urutan ID yang sama agar dua request
// yang menyentuh wallet yang sama tidak saling deadlock.
func lockLedger(tx *gorm.DB, userID uuid.UUID, ids ...uuid.UUID) (*walletLedger, error) {
	unique := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i].String() < unique[j].String() })

	repo := repository.NewWalletRepository(tx)
	ledger := &walletLedger{repo: repo, userID: userID, wallets: make(map[uuid.UUID]*model.Wallet, len(unique))}
	for _, id := range unique {
		wallet, err := repo.FindByIDForUpdate(id, userID)
		if err != nil {
			return nil, err
		}
		ledger.wallets[id] = wallet
	}
	return ledger, nil
}

func (l *walletLedger) wallet(id uuid.UUID) *model.Wallet {
	return l.wallets[id]
}

func (l *walletLedger) requireUsable(ids ...uuid.UUID) error {
	for _, id := range ids {
		w := l.wallets[id]
		if w.DeletedAt.Valid {
			return ErrWalletNotFound
		}
		if w.IsArchived {
			return ErrWalletArchived
		}
	}
	return nil
}

// apply menolak pengurangan yang membuat saldo minus, kecuali untuk kartu kredit.
// Penambahan saldo selalu diizinkan.
func (l *walletLedger) apply(id uuid.UUID, delta float64) error {
	w := l.wallets[id]
	if delta < 0 && !w.AllowsNegative() && w.Balance+delta < -balanceEpsilon {
		return ErrInsufficientBalance
	}
	if err := l.repo.AddBalance(id, l.userID, delta); err != nil {
		return err
	}
	w.Balance += delta
	return nil
}
