package dto

import (
	"time"

	"github.com/google/uuid"
)

// ── Request ───────────────────────────────────────────────────────────

type CreateNotificationRequest struct {
	UserID  uuid.UUID `json:"users_id" binding:"required"`
	Title   string    `json:"title"    binding:"required,max=100"`
	Message string    `json:"message"  binding:"required,max=255"`
}

// ── Response ──────────────────────────────────────────────────────────

type NotificationResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"users_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type NotificationListResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	TotalUnread   int64                  `json:"total_unread"`
}