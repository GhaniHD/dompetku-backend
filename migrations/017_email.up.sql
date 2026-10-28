-- Notifikasi email: verifikasi, ganti email, antrean pengiriman, dan preferensi.

ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verify_due_at timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS pending_email varchar(100) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_bounced_at timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_prefs jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE users ADD COLUMN IF NOT EXISTS weekly_sent_on date;
ALTER TABLE users ADD COLUMN IF NOT EXISTS reminder_sent_on date;

CREATE TABLE IF NOT EXISTS email_codes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email       varchar(100) NOT NULL,
    purpose     varchar(10) NOT NULL CHECK (purpose IN ('verify', 'change')),
    code_hash   varchar(64) NOT NULL,
    attempts    integer NOT NULL DEFAULT 0,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_email_codes_user ON email_codes (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS email_outbox (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid REFERENCES users(id) ON DELETE CASCADE,
    to_email         varchar(100) NOT NULL,
    subject          varchar(200) NOT NULL,
    html             text NOT NULL,
    text             text NOT NULL,
    kind             varchar(20) NOT NULL,
    priority         integer NOT NULL DEFAULT 1,
    unsubscribe      text NOT NULL DEFAULT '',
    status           varchar(10) NOT NULL DEFAULT 'pending',
    attempts         integer NOT NULL DEFAULT 0,
    next_attempt_at  timestamptz NOT NULL DEFAULT now(),
    last_error       varchar(300) NOT NULL DEFAULT '',
    sent_at          timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_email_outbox_queue ON email_outbox (priority, next_attempt_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_email_outbox_created ON email_outbox (created_at DESC);

-- Admin tidak perlu verifikasi email.
UPDATE users SET email_verified_at = COALESCE(email_verified_at, now()) WHERE role = 'admin';
