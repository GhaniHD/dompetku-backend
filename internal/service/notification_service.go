package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"

	"github.com/google/uuid"
)

const (
	defaultNotificationLimit = 20
)

type NotificationService interface {
	GetAll(userID uuid.UUID, filter dto.NotificationFilterRequest) (*dto.NotificationListResponse, error)
	MarkAsRead(id uuid.UUID, userID uuid.UUID) error
	MarkAllAsRead(userID uuid.UUID) error
	Delete(id uuid.UUID, userID uuid.UUID) error
	DeleteAll(userID uuid.UUID) error
	// CreateSystem dipakai service lain untuk membuat notifikasi internal.
	CreateSystem(userID uuid.UUID, title, message string) error
}

type notificationService struct {
	repo repository.NotificationRepository
}

func NewNotificationService(repo repository.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) GetAll(userID uuid.UUID, filter dto.NotificationFilterRequest) (*dto.NotificationListResponse, error) {
	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultNotificationLimit
	}

	notifications, err := s.repo.FindPage(userID, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountAll(userID)
	if err != nil {
		return nil, err
	}
	unread, err := s.repo.CountUnread(userID)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		responses = append(responses, toNotificationResponse(n))
	}

	return &dto.NotificationListResponse{
		Notifications: responses,
		TotalUnread:   unread,
		Total:         total,
		Page:          page,
		Limit:         limit,
	}, nil
}

func (s *notificationService) MarkAsRead(id uuid.UUID, userID uuid.UUID) error {
	affected, err := s.repo.MarkAsRead(id, userID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (s *notificationService) MarkAllAsRead(userID uuid.UUID) error {
	return s.repo.MarkAllAsRead(userID)
}

func (s *notificationService) Delete(id uuid.UUID, userID uuid.UUID) error {
	affected, err := s.repo.Delete(id, userID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (s *notificationService) DeleteAll(userID uuid.UUID) error {
	return s.repo.DeleteAllByUserID(userID)
}

func (s *notificationService) CreateSystem(userID uuid.UUID, title, message string) error {
	return s.repo.Create(&model.Notification{
		UserID:  userID,
		Title:   truncate(title, 100),
		Message: truncate(message, 255),
	})
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func toNotificationResponse(n model.Notification) dto.NotificationResponse {
	return dto.NotificationResponse{
		ID:        n.ID,
		UserID:    n.UserID,
		Title:     n.Title,
		Message:   n.Message,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}
}
