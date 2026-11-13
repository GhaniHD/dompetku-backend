# MyFulus — Backend

Backend API untuk **MyFulus** (sebelumnya Dompetku), aplikasi pencatatan keuangan pribadi
(personal finance tracker): wallet, transaksi, kategori, budget, laporan, dan
analisis keuangan berbasis AI. Dibangun dengan **Go + Gin + GORM +
PostgreSQL**, memakai **Uber FX** untuk dependency injection antar layer
(`handler → service → repository`).

## Fitur

- **Autentikasi**: register & login dengan JWT.
- **Multi-wallet** bertipe `cash`, `bank`, `ewallet`, `credit_card`, `savings`.
  Hanya kartu kredit yang boleh bersaldo minus. Wallet yang punya riwayat
  diarsipkan, bukan dihapus, sehingga laporan tetap utuh.
- **Penyesuaian saldo**: mencocokkan saldo aplikasi dengan saldo asli tanpa
  menimpa angka; selisihnya tercatat dan tidak masuk laporan.
- **Transaksi income/expense** terhubung ke wallet & kategori tertentu.
- **Transfer antar wallet** (misalnya Gopay → Cash) dengan biaya admin opsional.
  Transfer tidak dihitung sebagai pemasukan/pengeluaran; biaya admin dicatat
  sebagai pengeluaran kategori "Biaya Admin".
- **Kategori** custom per user (tipe `income`/`expense`).
- **Budget** bulanan per kategori, termasuk fitur **copy budget**. Periode budget
  mengikuti **tanggal gajian** user (`budget_start_day`, 1–31): dengan tanggal 25,
  budget "Oktober" mencakup 25 Okt – 24 Nov. Tanggal 29–31 pada bulan yang lebih
  pendek dipindah ke hari terakhir bulan itu.
- **Notifikasi** in-app dengan pagination. Notifikasi sistem hanya dikirim saat
  ambang baru terlewati: saldo turun di bawah Rp50.000 (kecuali kartu kredit),
  pengeluaran kategori mencapai 80% / 100% anggaran, serta transaksi besar.
- **Laporan** bulanan & tahunan. Laporan bulanan bisa per bulan kalender atau
  mengikuti siklus gajian (`period=cycle`).
- **Riwayat per dompet** seperti mutasi rekening: transaksi, transfer, dan
  penyesuaian dalam satu timeline beserta saldo setelah setiap entri.
- **Export CSV** transaksi dan transfer (UTF-8 dengan BOM, langsung terbuka di Excel).
- **Transaksi berulang** mingguan/bulanan (gaji, kos, langganan, cicilan). Penjadwal
  berjalan saat server start lalu setiap jam, mencatat jadwal yang terlewat, dan aman
  dijalankan di beberapa server sekaligus. Jika saldo tidak cukup, jadwal tetap maju
  dan user mendapat notifikasi.
- **Hutang & piutang** dengan pembayaran sebagian, pengingat H-1 dan hari H jatuh
  tempo. Pergerakannya mengubah saldo dompet tetapi tidak masuk laporan
  pemasukan/pengeluaran.
- **Target tabungan** yang terhubung ke satu dompet: progres, sisa, estimasi
  setoran per bulan sampai tenggat, dan notifikasi saat target tercapai.
- **Analisis keuangan berbasis AI** — mengirim ringkasan 6 bulan terakhir ke
  **Claude API** (`pkg/claude/client.go`) untuk insight dan prediksi 3 bulan.
  Hasil di-cache 24 jam selama datanya tidak berubah; tanpa `CLAUDE_API_KEY`
  endpoint tetap jalan dengan insight kosong.
- **Profil**: update profil, ganti password, upload avatar, hapus akun.
- **Persetujuan & undangan**: pendaftar baru menunggu persetujuan admin, kecuali
  memakai kode undangan. Satu akun hanya aktif di satu perangkat.
- **Email** lewat SMTP (Brevo): kode verifikasi (OTP), lupa password, ganti email,
  ringkasan mingguan, pengingat H-1, peringatan login perangkat baru. Dikirim
  lewat antrean dengan coba ulang dan batas 250 email/hari. Tanpa `SMTP_HOST`,
  email mati dan verifikasi tidak diwajibkan.
- **Konsol super admin** terpisah: persetujuan, pengguna, aktivitas login,
  undangan, pengumuman, statistik email. Admin tidak bisa melihat data keuangan.

## Tech Stack

| Komponen | Teknologi |
| --- | --- |
| Bahasa & framework | Go 1.25, Gin |
| ORM & database | GORM, PostgreSQL |
| Dependency Injection | Uber FX (`go.uber.org/fx`) |
| Auth | JWT (`golang-jwt/jwt/v5`), hashing password via `golang.org/x/crypto` |
| Integrasi AI | Anthropic Claude API (HTTP client custom di `pkg/claude`) |
| Deployment | VPS Ubuntu 24.04: binary Go + systemd, Caddy (HTTPS), PostgreSQL 17, GitHub Actions |
| Email | SMTP (Brevo), webhook bounce |

## Arsitektur

```
cmd/app/main.go              entry point, wiring seluruh module lewat Uber FX
api/
  routes.go                  registrasi semua route + Claude client provider
  handler/                   HTTP handler per resource (auth, wallet, transaction, dst)
internal/
  dto/                       request/response struct per resource
  model/                     GORM model (User, Wallet, Transaction, Category, Budget, Notification)
  repository/                akses database (GORM) per resource
  service/                   business logic per resource
  middleware/                auth middleware (JWT) & CORS middleware
pkg/
  claude/                    HTTP client tipis ke Anthropic Claude API (untuk fitur analisis)
  config/                    load environment variable ke struct AppConf
  crypto/                    helper hash & verifikasi password
  db/                        koneksi & inisialisasi GORM ke PostgreSQL
  utils/                     helper JWT (generate & verifikasi token)
```

Setiap layer (`handler`, `service`, `repository`) didaftarkan sebagai
provider Uber FX lewat `Module` masing-masing (lihat `internal/service/module.go`,
`internal/repository/module.go`, `api/handler/module.go`), lalu di-inject
otomatis ke `NewRouter` di `api/routes.go`.

## Menjalankan Proyek

### Opsi A — Docker Compose (pengembangan lokal)

```bash
cp .env.example .env   # sesuaikan isinya, lihat tabel environment variable
docker compose up --build
```

API akan jalan di `http://localhost:8090`.

### Opsi B — Manual (Go + PostgreSQL lokal)

```bash
cp .env.example .env
go mod download
go run ./cmd/app/main.go
```

Health check: `GET /health` → `OK`, atau `503 DB_DOWN` kalau database tidak menjawab dalam 2 detik.

## Environment Variable

| Variabel | Keterangan | Default |
| --- | --- | --- |
| `DB_HOST` | Host PostgreSQL | `db` |
| `DB_PORT` | Port PostgreSQL | `5432` |
| `DB_USER` | User database | `postgres` |
| `DB_PASSWORD` | Password database | `postgres` |
| `DB_NAME` | Nama database | `dompetku` |
| `DB_SSLMODE` | Mode SSL koneksi database | `disable` (di Docker Compose diset `require`) |
| `JWT_SECRET` | Secret untuk sign JWT, minimal 32 karakter. Server menolak start kalau kurang. | - |
| `PORT` / `SERVER_PORT` | Port HTTP server | `8090` |
| `BIND_ADDR` | Alamat dengar server. Di VPS diisi `127.0.0.1` agar hanya bisa dijangkau lewat Caddy | `0.0.0.0` |
| `ADMIN_EMAILS` | Email akun admin, dipisah koma | - |
| `CORS_ORIGINS` | Origin frontend yang diizinkan, dipisah koma | domain myfulus |
| `TRUSTED_PROXIES` | Proxy yang dipercaya untuk IP asli pengguna. Di VPS: `127.0.0.1,::1` | jaringan privat |
| `SMTP_HOST` | Host SMTP. Kosong = email mati | - |
| `SMTP_PORT` | Port SMTP (`2525` untuk Brevo) | `587` |
| `SMTP_USER` / `SMTP_PASS` | Login SMTP dan SMTP key | - |
| `MAIL_FROM` / `MAIL_REPLY_TO` | Pengirim dan alamat balasan | `MyFulus <noreply@myfulus.my.id>` / - |
| `APP_URL` | Alamat frontend, untuk link di email | `https://myfulus.my.id` |
| `API_URL` | Alamat publik backend tanpa `/api`, untuk tombol berhenti langganan satu klik | - |
| `BREVO_WEBHOOK_SECRET` | Secret di URL webhook Brevo (`?secret=`) | - |
| `ADMIN_WHATSAPP` | Nomor WA admin untuk tombol "Kabari admin" setelah daftar | - |
| `EMAIL_DOMAIN_CHECK` | Cek MX domain email saat daftar (`false` untuk mematikan) | `true` |
| `CLAUDE_API_KEY` | API key Anthropic untuk fitur analisis keuangan AI (`/api/analysis`) | - |
| `CLAUDE_MODEL` | Model Claude yang dipakai analisis | `claude-sonnet-4-20250514` |
| `CLAUDE_API_URL` | Endpoint Messages API (diganti hanya untuk test) | `https://api.anthropic.com/v1/messages` |

> File `.env` sudah ada di `.gitignore`, jadi kredensial asli tidak ikut
> ter-commit — cukup salin dari `.env.example` dan isi sendiri.

## Daftar Endpoint

Base URL: `/api`. Semua endpoint di bawah `/api` (kecuali `/api/auth/*`)
butuh header `Authorization: Bearer <token>`.

### Auth (publik)
| Method | Endpoint |
| --- | --- |
| POST | `/api/auth/register` |
| POST | `/api/auth/login` |

### Profile
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/profile` | Lihat profil |
| PUT | `/api/profile` | Update profil |
| PUT | `/api/profile/password` | Ganti password |
| POST | `/api/profile/avatar` | Upload avatar |
| DELETE | `/api/profile` | Hapus akun |
| PUT | `/api/profile/budget-cycle` | Atur tanggal mulai siklus budget. Body: `start_day` (1–31) |

### Wallet
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/wallets` | Buat wallet. Body: `name`, `type` (default `cash`), `balance` |
| GET | `/api/wallets?include_archived=true` | List wallet (arsip disembunyikan kecuali diminta) |
| GET | `/api/wallets/total-balance` | Total saldo semua wallet |
| GET | `/api/wallets/:id` | Detail wallet |
| PUT | `/api/wallets/:id` | Ubah `name`, `type`, `is_archived`. `balance` dicatat sebagai penyesuaian |
| DELETE | `/api/wallets/:id` | Hapus wallet tanpa riwayat; wallet dengan riwayat diarsipkan (saldo wajib 0) |
| POST | `/api/wallets/:id/adjust` | Penyesuaian saldo. Body: `actual_balance`, `note`, `date` (opsional) |
| GET | `/api/wallets/:id/adjustments` | Riwayat penyesuaian saldo |
| GET | `/api/wallets/:id/history?start_date=&end_date=&page=&limit=` | Mutasi dompet + `balance_after` per entri (terbaru dulu) |

### Transaksi
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/transactions` | Catat transaksi baru |
| GET | `/api/transactions` | List transaksi |
| GET | `/api/transactions/:id` | Detail transaksi |
| PUT | `/api/transactions/:id` | Update transaksi |
| DELETE | `/api/transactions/:id` | Hapus transaksi |

### Transfer
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/transfers` | Pindah saldo antar wallet. Body: `from_wallet_id`, `to_wallet_id`, `amount`, `fee` (opsional), `note`, `date` |
| GET | `/api/transfers?wallet_id=&start_date=&end_date=` | List transfer |
| GET | `/api/transfers/:id` | Detail transfer |
| DELETE | `/api/transfers/:id` | Hapus transfer, saldo dan biaya admin dikembalikan |

### Kategori
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/categories` | Buat kategori |
| GET | `/api/categories` | List kategori |
| GET | `/api/categories/:id` | Detail kategori |
| PUT | `/api/categories/:id` | Update kategori |
| DELETE | `/api/categories/:id` | Hapus kategori |

### Budget
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/budgets` | Buat budget |
| GET | `/api/budgets?month=&year=` | List budget beserta `period_start` / `period_end` sesuai siklus |
| GET | `/api/budgets/current` | Budget periode yang sedang berjalan sesuai tanggal gajian |
| GET | `/api/budgets/:id` | Detail budget |
| PUT | `/api/budgets/:id` | Update budget |
| DELETE | `/api/budgets/:id` | Hapus budget |
| POST | `/api/budgets/copy` | Duplikasi budget dari bulan/tahun lain |

### Notifikasi
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/notifications?page=1&limit=20` | List notifikasi (limit maks 100), beserta `total` dan `total_unread` |
| PATCH | `/api/notifications/read-all` | Tandai semua terbaca |
| PATCH | `/api/notifications/:id/read` | Tandai satu terbaca |
| DELETE | `/api/notifications/clear` | Hapus semua notifikasi |
| DELETE | `/api/notifications/:id` | Hapus satu notifikasi |

### Laporan
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/reports?month=&year=&period=calendar\|cycle` | Laporan bulanan, beserta `period_start` / `period_end` |
| GET | `/api/reports/yearly?year=` | Laporan tahunan |

### Transaksi Berulang
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/recurring` | Body: `wallet_id`, `category_id`, `type`, `amount`, `note`, `frequency` (`weekly`/`monthly`), `start_date`, `end_date` (opsional) |
| GET | `/api/recurring` | List jadwal beserta `next_run_date` dan `last_error` |
| GET | `/api/recurring/:id` | Detail jadwal |
| PUT | `/api/recurring/:id` | Ubah `wallet_id`, `category_id`, `amount`, `note`, `end_date`, `is_active` (jeda/lanjut) |
| DELETE | `/api/recurring/:id` | Hapus jadwal; transaksi yang sudah tercatat tetap ada |

### Hutang & Piutang
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/debts` | Body: `kind` (`payable` = hutang, `receivable` = piutang), `counterparty`, `amount`, `wallet_id` (opsional), `date`, `due_date`, `note` |
| GET | `/api/debts?kind=&status=open\|paid` | List + `total_payable` dan `total_receivable` (sisa yang belum lunas) |
| GET | `/api/debts/:id` | Detail beserta riwayat pembayaran |
| PUT | `/api/debts/:id` | Ubah `counterparty`, `note`, `due_date` |
| DELETE | `/api/debts/:id` | Hapus dan kembalikan semua pergerakan saldo |
| POST | `/api/debts/:id/payments` | Bayar/terima cicilan. Body: `amount`, `wallet_id` (opsional), `date`, `note` |
| DELETE | `/api/debts/:id/payments/:paymentId` | Batalkan satu pembayaran |

### Target Tabungan
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/goals` | Body: `name`, `target_amount`, `wallet_id`, `deadline` (opsional) |
| GET | `/api/goals` | List beserta `current_amount`, `progress_pct`, `monthly_needed` |
| GET | `/api/goals/:id` | Detail target |
| PUT | `/api/goals/:id` | Ubah `name`, `target_amount`, `deadline` |
| DELETE | `/api/goals/:id` | Hapus target (dompet dan saldonya tidak berubah) |

### Export
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/export/transactions.csv?start_date=&end_date=` | Unduh transaksi (urut kronologis) |
| GET | `/api/export/transfers.csv?start_date=&end_date=` | Unduh transfer |

### Analisis (AI)
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/analysis` | Analisis keuangan berbasis Claude API |

## Model Data (ringkas)

Semua entity pakai `uuid` sebagai primary key dan soft-delete (`gorm.DeletedAt`):

- **User**: `name`, `email` (unik), `password` (hash), `avatar_url`, `budget_start_day`.
- **Wallet**: `user_id`, `name`, `type`, `balance`, `opening_balance`, `is_archived`.
- **Category**: `user_id`, `name`, `type` (`income`/`expense`).
- **Transaction**: `user_id`, `wallet_id`, `category_id`, `amount`, `type`,
  `note`, `date`.
- **Budget**: `user_id`, `category_id`, `amount`, `month`, `year`, `notes`.
- **Notification**: `user_id`, `title`, `message`, `read`.
- **Transfer**: `user_id`, `from_wallet_id`, `to_wallet_id`, `amount`, `fee`,
  `fee_transaction_id`, `note`, `transfer_date`.
- **BalanceAdjustment**: `user_id`, `wallet_id`, `amount` (selisih bertanda),
  `previous_balance`, `new_balance`, `note`, `adjusted_at`.

## Migrasi Database

`AutoMigrate` hanya dipakai untuk tabel awal. Perubahan skema berikutnya ada di
folder `migrations/` dan **wajib dijalankan berurutan** di database (misalnya
dengan `psql -v ON_ERROR_STOP=1 -f`) **sebelum** kode yang membutuhkannya di-deploy.
Semua file aman dijalankan ulang.

| File | Isi |
| --- | --- |
| `001_widen_money_columns.up.sql` | Kolom uang jadi `numeric(18,2)`, tambah `wallets.opening_balance` |
| `002_create_transfers.up.sql` | Tabel `transfers` |
| `003_wallet_types_adjustments.up.sql` | Kolom `wallets.type`, `wallets.is_archived`, tabel `balance_adjustments` |
| `004_budget_cycle.up.sql` | Kolom `users.budget_start_day` (default 1 = bulan kalender) |
| `005_recurring_transactions.up.sql` | Tabel `recurring_transactions`, kolom `transactions.recurring_id` |
| `006_debts_goals.up.sql` | Tabel `debts`, `debt_movements`, `goals` |
| `007_admin_approval.up.sql` | Role, status, persetujuan admin, undangan, log admin |
| `008_income_type.up.sql` | Jenis pemasukan pengguna (gaji/tidak tetap) |
| `009_single_session.up.sql` | Satu akun satu perangkat |
| `010_savings_budget.up.sql` | Anggaran tabungan |
| `011_performance_indexes.up.sql` | Index query per pengguna dan tanggal |
| `012_foreign_key_indexes.up.sql` | Index kolom foreign key (mempercepat pulihkan backup) |
| `013_timestamptz.up.sql` | Semua kolom waktu jadi `timestamptz` |
| `014_daily_categories.up.sql` | Pilihan kategori jatah harian |
| `015_purge_deleted_users.up.sql` | Hapus permanen akun yang sudah dihapus |
| `016_super_admin.up.sql` | Aktivitas login, kehadiran online, pengumuman |
| `017_email.up.sql` | Verifikasi email, kode OTP, antrean email |
| `check_balance.sql` | Cek konsistensi saldo (hanya membaca). Hasil 0 baris = semua saldo cocok |

Invarian yang dijaga: `balance = opening_balance + Σ income − Σ expense + Σ transfer masuk − Σ transfer keluar + Σ penyesuaian + Σ pergerakan hutang/piutang`.

## Testing

Integration test di `tests/integration` menjalankan server sungguhan terhadap
PostgreSQL. Database test **dikosongkan setiap kali test jalan**, jadi pakai
database khusus yang namanya mengandung `test` (bukan salinan data penting).

Windows (PowerShell), memakai PostgreSQL lokal:

```powershell
& "C:\Program Files\PostgreSQL\17\bin\createdb.exe" -U postgres -p 5433 -W dompetku_ci_test

$env:TEST_DB_PORT = "5433"
$env:TEST_DB_PASSWORD = "<password postgres lokal>"
$env:TEST_DB_NAME = "dompetku_ci_test"
go test -count=1 ./...
```

Linux/macOS: `make test TEST_DB_PASSWORD=...`. Tanpa `TEST_DB_NAME`, integration
test otomatis dilewati.

GitHub Actions (`.github/workflows/test.yml`) menjalankan build, `go vet`, semua
test dengan PostgreSQL 17, dan `govulncheck` di setiap push.

## Deployment (produksi)

Produksi berjalan di satu VPS Ubuntu 24.04. Frontend ada di Vercel.

```
https://api.myfulus.my.id → Caddy (HTTPS) → 127.0.0.1:8090 (myfulus-api, systemd) → PostgreSQL 17 (localhost)
```

| Bagian | Lokasi |
| --- | --- |
| Program | `/opt/myfulus/releases/<versi>/myfulus-api`, versi aktif `/opt/myfulus/current` |
| Ganti versi + rollback otomatis | `/opt/myfulus/bin/activate <folder-versi>` (cek `/health` 30 detik) |
| Environment | `/etc/myfulus/api.env` (640 root:myfulus) |
| Upload | `/var/lib/myfulus/uploads` |
| Service | `myfulus-api.service`, log: `journalctl -u myfulus-api -f` |
| Backup | `myfulus-backup.timer` setiap 02.30 WIB, terenkripsi `age`, ke Google Drive |

**Deploy otomatis**: push ke `master` → workflow `test` lulus → workflow `deploy`
(`.github/workflows/deploy.yml`) membangun binary, mengirimnya ke VPS lewat user
`deploy`, lalu menjalankan `activate`. Kalau `/health` gagal, server kembali ke
versi sebelumnya dan workflow ditandai gagal. Secret yang dibutuhkan: `VPS_HOST`,
`VPS_SSH_KEY`, `VPS_KNOWN_HOSTS`.

**Rollback manual**: `ls -1dt /opt/myfulus/releases/*`, lalu
`sudo /opt/myfulus/bin/activate /opt/myfulus/releases/<versi-sebelumnya>`.

Panduan lengkap (setup server, migrasi data, backup, pemulihan) ada di dokumen
"Panduan Deploy MyFulus ke VPS" dan "Catatan Deploy MyFulus ke VPS".

## Catatan

- File `.env` dan `*.dump` (backup database) ada di `.gitignore`.
- Error internal dicatat di log server dengan format `[METHOD /path] pesan`;
  client hanya menerima pesan umum supaya detail database tidak bocor.
