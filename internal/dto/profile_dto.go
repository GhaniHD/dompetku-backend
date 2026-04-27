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

// ─── Response DTOs ────────────────────────────────────────────────────────────

// ProfileResponse adalah respons lengkap profil pengguna
type ProfileResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`

	// Statistik ringkas (dihitung dari relasi)
	TotalBalance float64 `json:"balance"`
	TotalTx      int64   `json:"transactions"`
	TotalWallet  int64   `json:"wallet"`
}

// AvatarResponse adalah respons setelah upload avatar berhasil
type AvatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}