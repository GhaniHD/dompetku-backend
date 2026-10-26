package dto

import (
	"time"

	"github.com/google/uuid"
)

type AdminUserFilter struct {
	Status string `form:"status" binding:"omitempty,oneof=pending active suspended rejected"`
	Query  string `form:"q"`
}

type AdminReasonRequest struct {
	Reason string `json:"reason" binding:"max=300"`
}

type AdminSummaryResponse struct {
	TotalUsers    int64 `json:"total_users"`
	Pending       int64 `json:"pending"`
	Active        int64 `json:"active"`
	Suspended     int64 `json:"suspended"`
	Rejected      int64 `json:"rejected"`
	Active30d     int64 `json:"active_30d"`
	New7d         int64 `json:"new_7d"`
	ActiveInvites int64 `json:"active_invites"`
}

// AdminUserResponse sengaja tidak memuat saldo atau isi transaksi user.
type AdminUserResponse struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	StatusReason string     `json:"status_reason"`
	CreatedAt    time.Time  `json:"created_at"`
	ApprovedAt   *time.Time `json:"approved_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	InviteCode   *string    `json:"invite_code"`
	Wallets      int64      `json:"wallets"`
	Transactions int64      `json:"transactions"`
}

type ResetLinkResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type CreateInviteRequest struct {
	Note      string `json:"note"       binding:"max=200"`
	MaxUses   int    `json:"max_uses"   binding:"omitempty,min=1,max=50"`
	ValidDays int    `json:"valid_days" binding:"omitempty,min=1,max=30"`
}

type InviteResponse struct {
	ID        uuid.UUID  `json:"id"`
	Code      string     `json:"code"`
	Note      string     `json:"note"`
	MaxUses   int        `json:"max_uses"`
	UsedCount int        `json:"used_count"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UsedBy    []string   `json:"used_by"`
}

type AdminLogResponse struct {
	ID          uuid.UUID `json:"id"`
	AdminName   string    `json:"admin_name"`
	Action      string    `json:"action"`
	TargetName  *string   `json:"target_name"`
	TargetEmail *string   `json:"target_email"`
	Detail      string    `json:"detail"`
	CreatedAt   time.Time `json:"created_at"`
}
