package service

import (
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationService interface {
	GetAll(userID uuid.UUID) (*dto.NotificationListResponse, error)
	MarkAsRead(id uuid.UUID, userID uuid.UUID) error
	MarkAllAsRead(userID uuid.UUID) error
	Delete(id uuid.UUID, userID uuid.UUID) error
	DeleteAll(userID uuid.UUID) error
	// CreateSystem digunakan oleh service lain (budget, transaksi, dll)
	// untuk membuat notifikasi secara internal.
	CreateSystem(userID uuid.UUID, title, message string) error
}

type notificationService struct {
	repo repository.NotificationRepository
}

func NewNotificationService(repo repository.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) GetAll(userID uuid.UUID) (*dto.NotificationListResponse, error) {
	notifications, err := s.repo.FindAllByUserID(userID)
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
	}, nil
}

func (s *notificationService) MarkAsRead(id uuid.UUID, userID uuid.UUID) error {
	notif, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("notifikasi tidak ditemukan")
		}
		return err
	}
	if notif.UserID != userID {
		return errors.New("akses ditolak")
	}
	return s.repo.MarkAsRead(id, userID)
}

func (s *notificationService) MarkAllAsRead(userID uuid.UUID) error {
	return s.repo.MarkAllAsRead(userID)
}

func (s *notificationService) Delete(id uuid.UUID, userID uuid.UUID) error {
	notif, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("notifikasi tidak ditemukan")
		}
		return err
	}
	if notif.UserID != userID {
		return errors.New("akses ditolak")
	}
	return s.repo.Delete(id, userID)
}

func (s *notificationService) DeleteAll(userID uuid.UUID) error {
	return s.repo.DeleteAllByUserID(userID)
}

func (s *notificationService) CreateSystem(userID uuid.UUID, title, message string) error {
	notif := &model.Notification{
		UserID:  userID,
		Title:   title,
		Message: message,
		IsRead:  false,
	}
	return s.repo.Create(notif)
}

// ── helper ────────────────────────────────────────────────────────────

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
