package dto

import (
	"time"

	"github.com/google/uuid"
)

type NotificationFilterRequest struct {
	Page  int `form:"page"  binding:"omitempty,min=1"`
	Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
}

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
	Total         int64                  `json:"total"`
	Page          int                    `json:"page"`
	Limit         int                    `json:"limit"`
}
