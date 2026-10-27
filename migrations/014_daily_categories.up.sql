-- Pilihan kategori untuk kartu jatah harian. Kosong = semua anggaran pengeluaran.
ALTER TABLE users ADD COLUMN IF NOT EXISTS daily_category_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
