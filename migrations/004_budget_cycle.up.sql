BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS budget_start_day smallint NOT NULL DEFAULT 1;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_budget_start_day_check') THEN
        ALTER TABLE users ADD CONSTRAINT users_budget_start_day_check
            CHECK (budget_start_day BETWEEN 1 AND 31);
    END IF;
END $$;

COMMIT;
