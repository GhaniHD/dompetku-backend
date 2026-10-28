package dto

import (
	"time"

	"github.com/google/uuid"
)

type AdminUserFilter struct {
	Status string `form:"status" binding:"omitempty,oneof=pending active suspended rejected"`
	Query  string `form:"q"`
	Online bool   `form:"online"`
}

type AdminReasonRequest struct {
	Reason string `json:"reason" binding:"max=300"`
}

type AdminPasswordRequest struct {
	Password string `json:"password" binding:"required,max=72"`
}

type HourCount struct {
	Hour  time.Time `json:"hour"`
	Users int64     `json:"users"`
}

type WeekCount struct {
	Week  time.Time `json:"week"`
	Count int64     `json:"count"`
}

// SuspiciousLogin: email dengan 5 atau lebih percobaan login gagal dalam 1 jam terakhir.
type SuspiciousLogin struct {
	Email    string    `json:"email"`
	Attempts int64     `json:"attempts"`
	IPs      int64     `json:"ips"`
	LastAt   time.Time `json:"last_at"`
}

type AdminSummaryResponse struct {
	TotalUsers     int64             `json:"total_users"`
	Pending        int64             `json:"pending"`
	Active         int64             `json:"active"`
	Suspended      int64             `json:"suspended"`
	Rejected       int64             `json:"rejected"`
	Active30d      int64             `json:"active_30d"`
	New7d          int64             `json:"new_7d"`
	ActiveInvites  int64             `json:"active_invites"`
	OnlineNow      int64             `json:"online_now"`
	LoginFailed24h int64             `json:"login_failed_24h"`
	Online24h      []HourCount       `json:"online_24h"`
	Registrations  []WeekCount       `json:"registrations"`
	Suspicious     []SuspiciousLogin `json:"suspicious"`
}

type AuthEventFilter struct {
	Event  string `form:"event"   binding:"omitempty,oneof=login login_failed login_locked logout password_changed password_reset forced_logout"`
	UserID string `form:"user_id" binding:"omitempty,uuid"`
	Query  string `form:"q"`
	Limit  int    `form:"limit"   binding:"omitempty,min=1,max=300"`
}

type AuthEventResponse struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"user_id"`
	UserName  *string    `json:"user_name"`
	Email     string     `json:"email"`
	Event     string     `json:"event"`
	Detail    string     `json:"detail"`
	IP        string     `json:"ip"`
	Device    string     `json:"device"`
	CreatedAt time.Time  `json:"created_at"`
}

type AdminUserDetail struct {
	User   AdminUserResponse   `json:"user"`
	Events []AuthEventResponse `json:"events"`
}

type AnnouncementRequest struct {
	Message string     `json:"message" binding:"required,min=1,max=280"`
	Level   string     `json:"level"   binding:"omitempty,oneof=info warning"`
	EndsAt  *time.Time `json:"ends_at"`
}

type AnnouncementResponse struct {
	ID        uuid.UUID  `json:"id"`
	Message   string     `json:"message"`
	Level     string     `json:"level"`
	EndsAt    *time.Time `json:"ends_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
}

// AdminUserResponse sengaja tidak memuat saldo atau isi transaksi user.
type AdminUserResponse struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	StatusReason    string     `json:"status_reason"`
	CreatedAt       time.Time  `json:"created_at"`
	ApprovedAt      *time.Time `json:"approved_at"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	LastSeenAt      *time.Time `json:"last_seen_at"`
	Online          bool       `json:"online"`
	LastLoginDevice string     `json:"last_login_device"`
	InviteCode      *string    `json:"invite_code"`
	Wallets         int64      `json:"wallets"`
	Transactions    int64      `json:"transactions"`
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
