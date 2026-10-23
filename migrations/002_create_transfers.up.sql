BEGIN;

CREATE TABLE IF NOT EXISTS transfers (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_wallet_id     uuid NOT NULL REFERENCES wallets(id),
    to_wallet_id       uuid NOT NULL REFERENCES wallets(id),
    amount             numeric(18,2) NOT NULL CHECK (amount > 0),
    fee                numeric(18,2) NOT NULL DEFAULT 0 CHECK (fee >= 0),
    fee_transaction_id uuid REFERENCES transactions(id),
    note               text NOT NULL DEFAULT '',
    transfer_date      date NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    CONSTRAINT transfers_different_wallets CHECK (from_wallet_id <> to_wallet_id)
);

CREATE INDEX IF NOT EXISTS idx_transfers_user_date ON transfers (user_id, transfer_date DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transfers_from_wallet ON transfers (from_wallet_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transfers_to_wallet ON transfers (to_wallet_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transfers_deleted_at ON transfers (deleted_at);

COMMIT;
