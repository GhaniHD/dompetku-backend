BEGIN;

DELETE FROM budgets WHERE kind = 'savings';
DROP INDEX IF EXISTS idx_budgets_savings_wallet;
ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_kind_check;
ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_goal_fk;
ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_wallet_fk;
ALTER TABLE budgets ALTER COLUMN category_id SET NOT NULL;
ALTER TABLE budgets DROP COLUMN IF EXISTS goal_id;
ALTER TABLE budgets DROP COLUMN IF EXISTS wallet_id;
ALTER TABLE budgets DROP COLUMN IF EXISTS kind;

COMMIT;
