BEGIN;

ALTER TABLE budgets ADD COLUMN IF NOT EXISTS kind varchar(10) NOT NULL DEFAULT 'expense';
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS wallet_id uuid;
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS goal_id uuid;
ALTER TABLE budgets ALTER COLUMN category_id DROP NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'budgets_wallet_fk') THEN
        ALTER TABLE budgets ADD CONSTRAINT budgets_wallet_fk FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE CASCADE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'budgets_goal_fk') THEN
        ALTER TABLE budgets ADD CONSTRAINT budgets_goal_fk FOREIGN KEY (goal_id) REFERENCES goals(id) ON DELETE SET NULL;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'budgets_kind_check') THEN
        ALTER TABLE budgets ADD CONSTRAINT budgets_kind_check CHECK (
            (kind = 'expense' AND category_id IS NOT NULL) OR (kind = 'savings' AND wallet_id IS NOT NULL)
        );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_budgets_savings_wallet ON budgets (user_id, wallet_id, year, month) WHERE kind = 'savings' AND deleted_at IS NULL;

COMMIT;
