package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExportEntry adalah satu pergerakan saldo dompet. Amount bertanda: positif
// menambah saldo, negatif mengurangi.
type ExportEntry struct {
	ID           uuid.UUID
	WalletID     uuid.UUID
	Source       string
	Kind         string
	Amount       float64
	Date         time.Time
	CreatedAt    time.Time
	CategoryName string
	Counterpart  string
	Note         string
	Recurring    bool
	TransferFee  bool
}

type ExportDebtMovement struct {
	ID           uuid.UUID
	DebtKind     string
	Counterparty string
	Kind         string
	Amount       float64
	WalletDelta  float64
	WalletName   *string
	Date         time.Time
	Note         string
}

type ExportRepository interface {
	// Entries mengembalikan semua pergerakan saldo pada rentang tanggal inklusif,
	// berurutan seperti riwayat dompet.
	Entries(userID uuid.UUID, start, end string) ([]ExportEntry, error)
	// BalancesBefore mengembalikan saldo tiap dompet sebelum tanggal start.
	BalancesBefore(userID uuid.UUID, start string) (map[uuid.UUID]float64, error)
	DebtMovements(userID uuid.UUID, start, end string) ([]ExportDebtMovement, error)
}

type exportRepository struct {
	db *gorm.DB
}

func NewExportRepository(db *gorm.DB) ExportRepository {
	return &exportRepository{db: db}
}

// userEntriesSQL harus tetap sejalan dengan walletEntriesSQL di history_repository.go.
const userEntriesSQL = `
	SELECT t.id, t.wallet_id, 'transaction' AS source, t.type AS kind,
	       CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END AS amount,
	       t.transaction_date AS date, t.created_at, COALESCE(c.name, '') AS category_name,
	       '' AS counterpart, t.note,
	       t.recurring_id IS NOT NULL AS recurring,
	       EXISTS (SELECT 1 FROM transfers f WHERE f.fee_transaction_id = t.id) AS transfer_fee
	FROM transactions t
	LEFT JOIN categories c ON c.id = t.category_id
	WHERE t.user_id = @user AND t.deleted_at IS NULL
	UNION ALL
	SELECT x.id, x.from_wallet_id, 'transfer', 'transfer_out', -x.amount, x.transfer_date, x.created_at,
	       '', w.name, x.note, false, false
	FROM transfers x JOIN wallets w ON w.id = x.to_wallet_id
	WHERE x.user_id = @user AND x.deleted_at IS NULL
	UNION ALL
	SELECT x.id, x.to_wallet_id, 'transfer', 'transfer_in', x.amount, x.transfer_date, x.created_at,
	       '', w.name, x.note, false, false
	FROM transfers x JOIN wallets w ON w.id = x.from_wallet_id
	WHERE x.user_id = @user AND x.deleted_at IS NULL
	UNION ALL
	SELECT a.id, a.wallet_id, 'adjustment', 'adjustment', a.amount, a.adjusted_at, a.created_at,
	       '', '', a.note, false, false
	FROM balance_adjustments a
	WHERE a.user_id = @user AND a.deleted_at IS NULL
	UNION ALL
	SELECT m.id, m.wallet_id, 'debt', CASE WHEN d.kind = 'payable' THEN 'debt_payable' ELSE 'debt_receivable' END,
	       m.wallet_delta, m.movement_date, m.created_at, '', d.counterparty, m.note, false, false
	FROM debt_movements m JOIN debts d ON d.id = m.debt_id
	WHERE m.user_id = @user AND m.deleted_at IS NULL AND m.wallet_id IS NOT NULL AND m.wallet_delta <> 0`

func (r *exportRepository) Entries(userID uuid.UUID, start, end string) ([]ExportEntry, error) {
	var rows []ExportEntry
	err := r.db.Raw(`SELECT * FROM (`+userEntriesSQL+`) e
		WHERE e.date >= CAST(@start AS date) AND e.date <= CAST(@end AS date)
		ORDER BY e.date, e.created_at, e.id`,
		map[string]any{"user": userID, "start": start, "end": end}).Scan(&rows).Error
	return rows, err
}

func (r *exportRepository) BalancesBefore(userID uuid.UUID, start string) (map[uuid.UUID]float64, error) {
	type row struct {
		ID      uuid.UUID
		Balance float64
	}
	var rows []row
	err := r.db.Raw(`SELECT w.id, w.opening_balance + COALESCE((
			SELECT SUM(e.amount) FROM (`+userEntriesSQL+`) e
			WHERE e.wallet_id = w.id AND e.date < CAST(@start AS date)
		), 0) AS balance
		FROM wallets w WHERE w.user_id = @user`,
		map[string]any{"user": userID, "start": start}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]float64, len(rows))
	for _, rw := range rows {
		out[rw.ID] = rw.Balance
	}
	return out, nil
}

func (r *exportRepository) DebtMovements(userID uuid.UUID, start, end string) ([]ExportDebtMovement, error) {
	var rows []ExportDebtMovement
	err := r.db.Raw(`SELECT m.id, d.kind AS debt_kind, d.counterparty, m.kind, m.amount, m.wallet_delta,
			w.name AS wallet_name, m.movement_date AS date, m.note
		FROM debt_movements m
		JOIN debts d ON d.id = m.debt_id
		LEFT JOIN wallets w ON w.id = m.wallet_id
		WHERE m.user_id = @user AND m.deleted_at IS NULL AND d.deleted_at IS NULL
		  AND m.movement_date >= CAST(@start AS date) AND m.movement_date <= CAST(@end AS date)
		ORDER BY m.movement_date, m.created_at`,
		map[string]any{"user": userID, "start": start, "end": end}).Scan(&rows).Error
	return rows, err
}
