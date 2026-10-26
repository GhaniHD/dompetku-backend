BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS income_type varchar(12) NOT NULL DEFAULT 'salary';
ALTER TABLE users ADD COLUMN IF NOT EXISTS income_setup_at timestamptz;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_income_type_check') THEN
        ALTER TABLE users ADD CONSTRAINT users_income_type_check CHECK (income_type IN ('salary', 'irregular'));
    END IF;
END $$;

-- Pengguna lama sudah memakai tanggal gajian, jadi tidak perlu ditanya lagi.
UPDATE users SET income_setup_at = created_at WHERE income_setup_at IS NULL;

COMMIT;
