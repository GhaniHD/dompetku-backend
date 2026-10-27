-- Index untuk query yang selalu difilter per pengguna dan tanggal.
-- Dijalankan tanpa BEGIN/COMMIT agar bisa dipakai di Supabase SQL Editor apa adanya.

CREATE INDEX IF NOT EXISTS idx_transactions_user_date ON transactions (user_id, transaction_date DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_date ON transactions (wallet_id, transaction_date) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transactions_user_category_date ON transactions (user_id, category_id, transaction_date) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transfers_fee_transaction ON transfers (fee_transaction_id) WHERE fee_transaction_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_balance_adjustments_user_date ON balance_adjustments (user_id, adjusted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debt_movements_user_date ON debt_movements (user_id, movement_date) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_wallets_user ON wallets (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_categories_user ON categories (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_budgets_user_period ON budgets (user_id, year, month) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications (user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email));

ANALYZE transactions, transfers, balance_adjustments, debt_movements, wallets, categories, budgets, notifications, users;
