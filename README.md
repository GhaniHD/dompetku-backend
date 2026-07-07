# Dompetku — Backend

Backend API untuk **Dompetku**, aplikasi pencatatan keuangan pribadi
(personal finance tracker): wallet, transaksi, kategori, budget, laporan, dan
analisis keuangan berbasis AI. Dibangun dengan **Go + Gin + GORM +
PostgreSQL**, memakai **Uber FX** untuk dependency injection antar layer
(`handler → service → repository`).

## Fitur

- **Autentikasi**: register & login dengan JWT.
- **Multi-wallet**: setiap user bisa punya beberapa dompet/rekening, lengkap
  dengan endpoint total saldo gabungan.
- **Transaksi income/expense** terhubung ke wallet & kategori tertentu.
- **Kategori** custom per user (tipe `income`/`expense`).
- **Budget** bulanan per kategori, termasuk fitur **copy budget** (duplikasi
  budget bulan sebelumnya ke bulan berjalan).
- **Notifikasi** in-app (read/unread, mark all as read, hapus semua/satu).
- **Laporan** bulanan & tahunan.
- **Analisis keuangan berbasis AI** — mengirim data keuangan user ke
  **Claude API** (`pkg/claude/client.go`, model `claude-sonnet-4-20250514`)
  untuk menghasilkan insight/analisis dalam bahasa natural.
- **Profil**: update profil, ganti password, upload avatar, hapus akun.

## Tech Stack

| Komponen | Teknologi |
| --- | --- |
| Bahasa & framework | Go 1.25, Gin |
| ORM & database | GORM, PostgreSQL |
| Dependency Injection | Uber FX (`go.uber.org/fx`) |
| Auth | JWT (`golang-jwt/jwt/v5`), hashing password via `golang.org/x/crypto` |
| Integrasi AI | Anthropic Claude API (HTTP client custom di `pkg/claude`) |
| Deployment | Docker (multi-stage build) + Docker Compose |

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

### Opsi A — Docker Compose (disarankan)

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

Health check: `GET /health` → `OK`.

## Environment Variable

| Variabel | Keterangan | Default |
| --- | --- | --- |
| `DB_HOST` | Host PostgreSQL | `db` |
| `DB_PORT` | Port PostgreSQL | `5432` |
| `DB_USER` | User database | `postgres` |
| `DB_PASSWORD` | Password database | `postgres` |
| `DB_NAME` | Nama database | `dompetku` |
| `DB_SSLMODE` | Mode SSL koneksi database | `disable` (di Docker Compose diset `require`) |
| `JWT_SECRET` | Secret untuk sign JWT. **Wajib diganti di production.** | `your-secret-key` |
| `PORT` / `SERVER_PORT` | Port HTTP server | `8090` |
| `CLAUDE_API_KEY` | API key Anthropic untuk fitur analisis keuangan AI (`/api/analysis`) | - |

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

### Wallet
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/wallets` | Buat wallet baru |
| GET | `/api/wallets` | List wallet milik user |
| GET | `/api/wallets/total-balance` | Total saldo semua wallet |
| GET | `/api/wallets/:id` | Detail wallet |
| PUT | `/api/wallets/:id` | Update wallet |
| DELETE | `/api/wallets/:id` | Hapus wallet |

### Transaksi
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| POST | `/api/transactions` | Catat transaksi baru |
| GET | `/api/transactions` | List transaksi |
| GET | `/api/transactions/:id` | Detail transaksi |
| PUT | `/api/transactions/:id` | Update transaksi |
| DELETE | `/api/transactions/:id` | Hapus transaksi |

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
| GET | `/api/budgets` | List budget |
| GET | `/api/budgets/:id` | Detail budget |
| PUT | `/api/budgets/:id` | Update budget |
| DELETE | `/api/budgets/:id` | Hapus budget |
| POST | `/api/budgets/copy` | Duplikasi budget dari bulan/tahun lain |

### Notifikasi
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/notifications` | List notifikasi |
| PATCH | `/api/notifications/read-all` | Tandai semua terbaca |
| PATCH | `/api/notifications/:id/read` | Tandai satu terbaca |
| DELETE | `/api/notifications/clear` | Hapus semua notifikasi |
| DELETE | `/api/notifications/:id` | Hapus satu notifikasi |

### Laporan
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/reports?month=&year=` | Laporan bulanan |
| GET | `/api/reports/yearly?year=` | Laporan tahunan |

### Analisis (AI)
| Method | Endpoint | Keterangan |
| --- | --- | --- |
| GET | `/api/analysis` | Analisis keuangan berbasis Claude API |

## Model Data (ringkas)

Semua entity pakai `uuid` sebagai primary key dan soft-delete (`gorm.DeletedAt`):

- **User**: `name`, `email` (unik), `password` (hash), `avatar_url`.
- **Wallet**: `user_id`, `name`, `balance`.
- **Category**: `user_id`, `name`, `type` (`income`/`expense`).
- **Transaction**: `user_id`, `wallet_id`, `category_id`, `amount`, `type`,
  `note`, `date`.
- **Budget**: `user_id`, `category_id`, `amount`, `month`, `year`, `notes`.
- **Notification**: `user_id`, `title`, `message`, `read`.

## Catatan

- Migrasi skema database dilakukan lewat GORM auto-migrate (lihat
  `pkg/db/database.go`) — tidak perlu tool migration terpisah.
- `pkg/config/config.go` saat ini mencetak beberapa nilai environment
  (`DB_HOST`, `DB_PASSWORD`, `DB_USER`) ke log sebagai debug — sebaiknya
  dihapus atau dibungkus flag debug sebelum deploy ke production, supaya
  kredensial tidak bocor ke log server.
