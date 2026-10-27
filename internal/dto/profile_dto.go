// internal/dto/profile_dto.go
package dto

import (
	"time"

	"github.com/google/uuid"
)

// ─── Request DTOs ─────────────────────────────────────────────────────────────

// UpdateProfileRequest digunakan untuk PUT /api/profile
type UpdateProfileRequest struct {
	Name  string `json:"name"  binding:"required,min=2,max=100"`
	Email string `json:"email" binding:"required,email"`
}

// ChangePasswordRequest digunakan untuk PUT /api/profile/password
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// UpdateBudgetCycleRequest digunakan untuk PUT /api/profile/budget-cycle
type UpdateBudgetCycleRequest struct {
	StartDay int `json:"start_day" binding:"required,min=1,max=31"`
}

// UpdateIncomeRequest digunakan untuk PUT /api/profile/income.
// StartDay wajib untuk salary; diabaikan untuk irregular (periode = bulan kalender).
type UpdateIncomeRequest struct {
	IncomeType string `json:"income_type" binding:"required,oneof=salary irregular"`
	StartDay   int    `json:"start_day"   binding:"omitempty,min=1,max=31"`
}

// UpdateDailyCategoriesRequest digunakan untuk PUT /api/profile/daily-categories.
// Daftar kosong berarti jatah harian memakai semua anggaran pengeluaran.
type UpdateDailyCategoriesRequest struct {
	CategoryIDs []uuid.UUID `json:"category_ids" binding:"max=20"`
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

// ProfileResponse adalah respons lengkap profil pengguna
type ProfileResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`

	BudgetStartDay  int    `json:"budget_start_day"`
	IncomeType      string `json:"income_type"`
	IncomeSetupDone bool   `json:"income_setup_done"`
	// DailyCategoryIDs: kategori kartu jatah harian; kosong = semua anggaran.
	DailyCategoryIDs []uuid.UUID `json:"daily_category_ids"`
	LastLoginAt      *time.Time  `json:"last_login_at"`
	LastLoginDevice  string      `json:"last_login_device"`
	Role             string      `json:"role"`
	Status           string      `json:"status"`

	// Statistik ringkas (dihitung dari relasi)
	TotalBalance float64 `json:"balance"`
	TotalTx      int64   `json:"transactions"`
	TotalWallet  int64   `json:"wallet"`
}

// AvatarResponse adalah respons setelah upload avatar berhasil
type AvatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}
