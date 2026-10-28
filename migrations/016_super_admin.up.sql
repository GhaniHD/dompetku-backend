-- Konsol super admin: log aktivitas akun, penanda online, dan pengumuman.

ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at timestamptz;

CREATE TABLE IF NOT EXISTS auth_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid REFERENCES users(id) ON DELETE CASCADE,
    email       varchar(100) NOT NULL DEFAULT '',
    event       varchar(30) NOT NULL,
    detail      varchar(200) NOT NULL DEFAULT '',
    ip          varchar(64) NOT NULL DEFAULT '',
    device      varchar(120) NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_auth_events_created ON auth_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_events_user ON auth_events (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_events_failed ON auth_events (created_at DESC) WHERE event = 'login_failed';

CREATE TABLE IF NOT EXISTS presence_hours (
    hour     timestamptz NOT NULL,
    user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (hour, user_id)
);
CREATE INDEX IF NOT EXISTS idx_presence_hours_user ON presence_hours (user_id);

CREATE TABLE IF NOT EXISTS announcements (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    message     text NOT NULL,
    level       varchar(10) NOT NULL DEFAULT 'info' CHECK (level IN ('info', 'warning')),
    ends_at     timestamptz,
    ended_at    timestamptz,
    created_by  uuid NOT NULL REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
