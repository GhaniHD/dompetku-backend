-- Hanya membaca data. Hasil 0 baris = semua saldo dompet konsisten.
WITH expected AS (
    SELECT w.id, w.name, w.balance,
           w.opening_balance
           + COALESCE((SELECT SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END)
                       FROM transactions t WHERE t.wallet_id = w.id AND t.deleted_at IS NULL), 0)
           + COALESCE((SELECT SUM(x.amount) FROM transfers x WHERE x.to_wallet_id = w.id AND x.deleted_at IS NULL), 0)
           - COALESCE((SELECT SUM(x.amount) FROM transfers x WHERE x.from_wallet_id = w.id AND x.deleted_at IS NULL), 0)
           + COALESCE((SELECT SUM(a.amount) FROM balance_adjustments a WHERE a.wallet_id = w.id AND a.deleted_at IS NULL), 0)
           + COALESCE((SELECT SUM(m.wallet_delta) FROM debt_movements m WHERE m.wallet_id = w.id AND m.deleted_at IS NULL), 0)
           AS expected_balance
    FROM wallets w
    WHERE w.deleted_at IS NULL
)
SELECT * FROM expected WHERE balance <> expected_balance;
