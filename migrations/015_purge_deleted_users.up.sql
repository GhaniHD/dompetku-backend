-- Akun yang dulu hanya disembunyikan (soft delete) dihapus permanen beserta
-- seluruh datanya, sehingga emailnya bisa dipakai mendaftar lagi.
DELETE FROM budgets WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM goals WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM debt_movements WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM debts WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM transfers WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM balance_adjustments WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM transactions WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM recurring_transactions WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM wallets WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM categories WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin');
DELETE FROM users WHERE deleted_at IS NOT NULL AND role <> 'admin';
