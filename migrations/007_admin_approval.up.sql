BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS role varchar(10) NOT NULL DEFAULT 'user';
ALTER TABLE users ADD COLUMN IF NOT EXISTS status varchar(12) NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS status_reason text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS approved_at timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS approved_by uuid;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version integer NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS invite_id uuid;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_role_check') THEN
        ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('user', 'admin'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_status_check') THEN
        ALTER TABLE users ADD CONSTRAINT users_status_check CHECK (status IN ('pending', 'active', 'suspended', 'rejected'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_status ON users (status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS invites (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        varchar(32) NOT NULL UNIQUE,
    note        text NOT NULL DEFAULT '',
    max_uses    integer NOT NULL DEFAULT 1 CHECK (max_uses > 0),
    used_count  integer NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    created_by  uuid NOT NULL REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  varchar(64) NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_by  uuid REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens (user_id);

CREATE TABLE IF NOT EXISTS admin_logs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id        uuid NOT NULL REFERENCES users(id),
    action          varchar(40) NOT NULL,
    target_user_id  uuid REFERENCES users(id) ON DELETE SET NULL,
    detail          text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_admin_logs_created ON admin_logs (created_at DESC);

COMMIT;
