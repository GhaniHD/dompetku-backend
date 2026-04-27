package repository

import (
	"dompetku/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationRepository interface {
	Create(notification *model.Notification) error
	FindAllByUserID(userID uuid.UUID) ([]model.Notification, error)
	FindByID(id uuid.UUID) (*model.Notification, error)
	MarkAsRead(id uuid.UUID, userID uuid.UUID) error
	MarkAllAsRead(userID uuid.UUID) error
	Delete(id uuid.UUID, userID uuid.UUID) error
	DeleteAllByUserID(userID uuid.UUID) error
	CountUnread(userID uuid.UUID) (int64, error)
}

type notificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(notification *model.Notification) error {
	return r.db.Create(notification).Error
}

func (r *notificationRepository) FindAllByUserID(userID uuid.UUID) ([]model.Notification, error) {
	var notifications []model.Notification
	err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&notifications).Error
	return notifications, err
}

func (r *notificationRepository) FindByID(id uuid.UUID) (*model.Notification, error) {
	var notification model.Notification
	err := r.db.Where("id = ?", id).First(&notification).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (r *notificationRepository) MarkAsRead(id uuid.UUID, userID uuid.UUID) error {
	return r.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true).Error
}

func (r *notificationRepository) MarkAllAsRead(userID uuid.UUID) error {
	return r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Update("is_read", true).Error
}

func (r *notificationRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	return r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Notification{}).Error
}

func (r *notificationRepository) DeleteAllByUserID(userID uuid.UUID) error {
	return r.db.
		Where("user_id = ?", userID).
		Delete(&model.Notification{}).Error
}

func (r *notificationRepository) CountUnread(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Count(&count).Error
	return count, err
}