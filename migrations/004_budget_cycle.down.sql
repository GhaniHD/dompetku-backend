BEGIN;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_budget_start_day_check;
ALTER TABLE users DROP COLUMN IF EXISTS budget_start_day;

COMMIT;
