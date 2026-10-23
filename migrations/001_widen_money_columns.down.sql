BEGIN;

ALTER TABLE wallets DROP COLUMN IF EXISTS opening_balance;

ALTER TABLE budgets      ALTER COLUMN amount  TYPE decimal(10,2);
ALTER TABLE transactions ALTER COLUMN amount  TYPE decimal(10,2);
ALTER TABLE wallets      ALTER COLUMN balance TYPE decimal(10,2);

COMMIT;
