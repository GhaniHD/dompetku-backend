BEGIN;

CREATE TABLE IF NOT EXISTS debts (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind              varchar(10) NOT NULL CHECK (kind IN ('payable', 'receivable')),
    counterparty      varchar(100) NOT NULL,
    principal         numeric(18,2) NOT NULL CHECK (principal > 0),
    paid_amount       numeric(18,2) NOT NULL DEFAULT 0 CHECK (paid_amount >= 0),
    note              text NOT NULL DEFAULT '',
    due_date          date,
    last_reminded_on  date,
    created_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz,
    CONSTRAINT debts_not_overpaid CHECK (paid_amount <= principal)
);

CREATE INDEX IF NOT EXISTS idx_debts_user ON debts (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debts_due ON debts (due_date) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debts_deleted_at ON debts (deleted_at);

CREATE TABLE IF NOT EXISTS debt_movements (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    debt_id        uuid NOT NULL REFERENCES debts(id) ON DELETE CASCADE,
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id      uuid REFERENCES wallets(id),
    kind           varchar(10) NOT NULL CHECK (kind IN ('principal', 'payment')),
    amount         numeric(18,2) NOT NULL CHECK (amount > 0),
    wallet_delta   numeric(18,2) NOT NULL DEFAULT 0,
    movement_date  date NOT NULL,
    note           text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);

CREATE INDEX IF NOT EXISTS idx_debt_movements_debt ON debt_movements (debt_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debt_movements_wallet ON debt_movements (wallet_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debt_movements_deleted_at ON debt_movements (deleted_at);

CREATE TABLE IF NOT EXISTS goals (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id      uuid NOT NULL REFERENCES wallets(id),
    name           varchar(100) NOT NULL,
    target_amount  numeric(18,2) NOT NULL CHECK (target_amount > 0),
    deadline       date,
    achieved_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_goals_wallet_active ON goals (wallet_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_goals_user ON goals (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_goals_deleted_at ON goals (deleted_at);

COMMIT;
