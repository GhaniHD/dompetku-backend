package dto

import (
	"time"

	"github.com/google/uuid"
)

// DTO Response
type CategoryResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// DTO untuk Create Category
type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required,oneof=income expense"`
}

// DTO untuk Update Category
type UpdateCategoryRequest struct {
	Name *string `json:"name"`
	Type *string `json:"type" binding:"omitempty,oneof=income expense"`
}

// DTO Response dengan User
type CategoryWithUserResponse struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	CreatedAt time.Time    `json:"created_at"`
	User      UserResponse `json:"user"`
}
