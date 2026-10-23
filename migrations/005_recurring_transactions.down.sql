BEGIN;

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_recurring_id_fkey;
DROP INDEX IF EXISTS idx_transactions_recurring_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS recurring_id;
DROP TABLE IF EXISTS recurring_transactions;

COMMIT;
