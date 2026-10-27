-- Beberapa tabel lama di production masih memakai "timestamp without time zone"
-- untuk created_at. Nilainya tersimpan dalam UTC, tetapi terbaca sebagai WIB
-- sehingga waktu tampil mundur 7 jam. Semua kolom seperti itu diubah ke
-- timestamptz dengan menandai nilai lamanya sebagai UTC.

DO $$
DECLARE
  col record;
BEGIN
  FOR col IN
    SELECT table_name, column_name
    FROM information_schema.columns
    WHERE table_schema = 'public' AND data_type = 'timestamp without time zone'
  LOOP
    EXECUTE format(
      'ALTER TABLE %I ALTER COLUMN %I TYPE timestamptz USING %I AT TIME ZONE ''UTC''',
      col.table_name, col.column_name, col.column_name
    );
  END LOOP;
END $$;
