BEGIN;

ALTER TABLE wallets ADD COLUMN IF NOT EXISTS type varchar(20) NOT NULL DEFAULT 'cash';
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS is_archived boolean NOT NULL DEFAULT false;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'wallets_type_check') THEN
        ALTER TABLE wallets ADD CONSTRAINT wallets_type_check
            CHECK (type IN ('cash', 'bank', 'ewallet', 'credit_card', 'savings'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS balance_adjustments (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id        uuid NOT NULL REFERENCES wallets(id),
    amount           numeric(18,2) NOT NULL CHECK (amount <> 0),
    previous_balance numeric(18,2) NOT NULL,
    new_balance      numeric(18,2) NOT NULL,
    note             text NOT NULL DEFAULT '',
    adjusted_at      date NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz
);

CREATE INDEX IF NOT EXISTS idx_balance_adjustments_wallet ON balance_adjustments (wallet_id, adjusted_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_balance_adjustments_deleted_at ON balance_adjustments (deleted_at);

COMMIT;
