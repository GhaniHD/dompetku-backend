package repository

import (
	"dompetku/internal/dto"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type HistoryRepository interface {
	WalletHistory(walletID uuid.UUID, openingBalance float64, filter dto.WalletHistoryFilterRequest, limit, offset int) ([]dto.WalletHistoryEntry, int64, error)
}

type historyRepository struct {
	db *gorm.DB
}

func NewHistoryRepository(db *gorm.DB) HistoryRepository {
	return &historyRepository{db: db}
}

// walletEntriesSQL menyatukan semua pergerakan saldo satu wallet. Setiap sumber
// baru (misalnya hutang) wajib ditambahkan di sini agar saldo berjalan tetap benar.
const walletEntriesSQL = `
	SELECT t.id, 'transaction' AS source, t.type AS kind,
	       CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END AS amount,
	       t.transaction_date AS date, t.created_at, c.name AS category_name,
	       '' AS counterpart, t.note
	FROM transactions t
	LEFT JOIN categories c ON c.id = t.category_id
	WHERE t.wallet_id = @wallet AND t.deleted_at IS NULL
	UNION ALL
	SELECT x.id, 'transfer', 'transfer_out', -x.amount, x.transfer_date, x.created_at,
	       '', w.name, x.note
	FROM transfers x JOIN wallets w ON w.id = x.to_wallet_id
	WHERE x.from_wallet_id = @wallet AND x.deleted_at IS NULL
	UNION ALL
	SELECT x.id, 'transfer', 'transfer_in', x.amount, x.transfer_date, x.created_at,
	       '', w.name, x.note
	FROM transfers x JOIN wallets w ON w.id = x.from_wallet_id
	WHERE x.to_wallet_id = @wallet AND x.deleted_at IS NULL
	UNION ALL
	SELECT a.id, 'adjustment', 'adjustment', a.amount, a.adjusted_at, a.created_at,
	       '', '', a.note
	FROM balance_adjustments a
	WHERE a.wallet_id = @wallet AND a.deleted_at IS NULL
	UNION ALL
	SELECT m.id, 'debt', CASE WHEN d.kind = 'payable' THEN 'debt_payable' ELSE 'debt_receivable' END,
	       m.wallet_delta, m.movement_date, m.created_at, '', d.counterparty, m.note
	FROM debt_movements m JOIN debts d ON d.id = m.debt_id
	WHERE m.wallet_id = @wallet AND m.deleted_at IS NULL AND m.wallet_delta <> 0`

func (r *historyRepository) WalletHistory(walletID uuid.UUID, openingBalance float64, filter dto.WalletHistoryFilterRequest, limit, offset int) ([]dto.WalletHistoryEntry, int64, error) {
	args := map[string]any{
		"wallet":  walletID,
		"opening": openingBalance,
		"start":   "0001-01-01",
		"end":     "9999-12-31",
		"limit":   limit,
		"offset":  offset,
	}
	if filter.StartDate != nil {
		args["start"] = filter.StartDate.Format("2006-01-02")
	}
	if filter.EndDate != nil {
		args["end"] = filter.EndDate.Format("2006-01-02")
	}

	base := `WITH entries AS (` + walletEntriesSQL + `),
	running AS (
		SELECT e.*, @opening + SUM(e.amount) OVER (
			ORDER BY e.date, e.created_at, e.id ROWS UNBOUNDED PRECEDING
		) AS balance_after
		FROM entries e
	)`
	where := ` FROM running WHERE date >= CAST(@start AS date) AND date <= CAST(@end AS date)`

	var total int64
	if err := r.db.Raw(base+` SELECT COUNT(*)`+where, args).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	type row struct {
		ID           uuid.UUID
		Source       string
		Kind         string
		Amount       float64
		Date         time.Time
		CreatedAt    time.Time
		CategoryName *string
		Counterpart  string
		Note         string
		BalanceAfter float64
	}
	var rows []row
	err := r.db.Raw(base+` SELECT *`+where+`
		ORDER BY date DESC, created_at DESC, id DESC
		LIMIT @limit OFFSET @offset`, args).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	entries := make([]dto.WalletHistoryEntry, 0, len(rows))
	for _, rw := range rows {
		category := ""
		if rw.CategoryName != nil {
			category = *rw.CategoryName
		}
		entries = append(entries, dto.WalletHistoryEntry{
			ID:           rw.ID,
			Source:       rw.Source,
			Kind:         rw.Kind,
			Amount:       rw.Amount,
			Date:         rw.Date.Format("2006-01-02"),
			CategoryName: category,
			Counterpart:  rw.Counterpart,
			Note:         rw.Note,
			BalanceAfter: rw.BalanceAfter,
			CreatedAt:    rw.CreatedAt,
		})
	}
	return entries, total, nil
}
