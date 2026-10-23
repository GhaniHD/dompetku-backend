BEGIN;

ALTER TABLE wallets      ALTER COLUMN balance TYPE numeric(18,2);
ALTER TABLE transactions ALTER COLUMN amount  TYPE numeric(18,2);
ALTER TABLE budgets      ALTER COLUMN amount  TYPE numeric(18,2);

ALTER TABLE wallets ADD COLUMN IF NOT EXISTS opening_balance numeric(18,2) NOT NULL DEFAULT 0;

UPDATE wallets w
SET opening_balance = w.balance - COALESCE((
    SELECT SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END)
    FROM transactions t
    WHERE t.wallet_id = w.id AND t.deleted_at IS NULL
), 0);

COMMIT;
