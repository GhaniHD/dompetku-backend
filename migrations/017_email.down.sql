DROP TABLE IF EXISTS email_outbox;
DROP TABLE IF EXISTS email_codes;
ALTER TABLE users DROP COLUMN IF EXISTS reminder_sent_on;
ALTER TABLE users DROP COLUMN IF EXISTS weekly_sent_on;
ALTER TABLE users DROP COLUMN IF EXISTS email_prefs;
ALTER TABLE users DROP COLUMN IF EXISTS email_bounced_at;
ALTER TABLE users DROP COLUMN IF EXISTS pending_email;
ALTER TABLE users DROP COLUMN IF EXISTS email_verify_due_at;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
