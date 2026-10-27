-- Index penuh untuk kolom foreign key. Dipakai PostgreSQL saat menghapus dompet,
-- kategori, atau akun (pulihkan backup, hapus akun); index parsial
-- "WHERE deleted_at IS NULL" tidak bisa dipakai untuk pengecekan ini.

CREATE INDEX IF NOT EXISTS idx_transactions_wallet_fk ON transactions (wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_category_fk ON transactions (category_id);
CREATE INDEX IF NOT EXISTS idx_transfers_from_wallet_fk ON transfers (from_wallet_id);
CREATE INDEX IF NOT EXISTS idx_transfers_to_wallet_fk ON transfers (to_wallet_id);
CREATE INDEX IF NOT EXISTS idx_balance_adjustments_wallet_fk ON balance_adjustments (wallet_id);
CREATE INDEX IF NOT EXISTS idx_debt_movements_wallet_fk ON debt_movements (wallet_id) WHERE wallet_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_budgets_category_fk ON budgets (category_id) WHERE category_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_budgets_wallet_fk ON budgets (wallet_id) WHERE wallet_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_budgets_goal_fk ON budgets (goal_id) WHERE goal_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_goals_wallet_fk ON goals (wallet_id);
CREATE INDEX IF NOT EXISTS idx_recurring_wallet_fk ON recurring_transactions (wallet_id);
CREATE INDEX IF NOT EXISTS idx_recurring_category_fk ON recurring_transactions (category_id);
