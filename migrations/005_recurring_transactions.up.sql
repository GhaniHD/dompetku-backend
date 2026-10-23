BEGIN;

CREATE TABLE IF NOT EXISTS recurring_transactions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id     uuid NOT NULL REFERENCES wallets(id),
    category_id   uuid NOT NULL REFERENCES categories(id),
    type          varchar(10) NOT NULL CHECK (type IN ('income', 'expense')),
    amount        numeric(18,2) NOT NULL CHECK (amount > 0),
    note          text NOT NULL DEFAULT '',
    frequency     varchar(10) NOT NULL CHECK (frequency IN ('weekly', 'monthly')),
    anchor_day    smallint NOT NULL CHECK (anchor_day BETWEEN 1 AND 31),
    start_date    date NOT NULL,
    end_date      date,
    next_run_date date NOT NULL,
    is_active     boolean NOT NULL DEFAULT true,
    last_run_at   timestamptz,
    last_error    text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

CREATE INDEX IF NOT EXISTS idx_recurring_due ON recurring_transactions (next_run_date)
    WHERE is_active AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_recurring_user ON recurring_transactions (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_recurring_deleted_at ON recurring_transactions (deleted_at);

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS recurring_id uuid;
CREATE INDEX IF NOT EXISTS idx_transactions_recurring_id ON transactions (recurring_id);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_recurring_id_fkey') THEN
        ALTER TABLE transactions ADD CONSTRAINT transactions_recurring_id_fkey
            FOREIGN KEY (recurring_id) REFERENCES recurring_transactions(id);
    END IF;
END $$;

COMMIT;
