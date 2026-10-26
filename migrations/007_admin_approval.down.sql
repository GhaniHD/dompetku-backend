BEGIN;

DROP TABLE IF EXISTS admin_logs;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS invites;

DROP INDEX IF EXISTS idx_users_status;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_check;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users
    DROP COLUMN IF EXISTS invite_id,
    DROP COLUMN IF EXISTS token_version,
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS status_reason,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS role;

COMMIT;
