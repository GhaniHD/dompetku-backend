package repository

import (
	"dompetku/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationRepository interface {
	Create(notification *model.Notification) error
	FindPage(userID uuid.UUID, limit, offset int) ([]model.Notification, error)
	CountAll(userID uuid.UUID) (int64, error)
	CountUnread(userID uuid.UUID) (int64, error)
	// MarkAsRead dan Delete mengembalikan jumlah baris yang terkena; 0 berarti
	// notifikasi tidak ada atau bukan milik user.
	MarkAsRead(id uuid.UUID, userID uuid.UUID) (int64, error)
	MarkAllAsRead(userID uuid.UUID) error
	Delete(id uuid.UUID, userID uuid.UUID) (int64, error)
	DeleteAllByUserID(userID uuid.UUID) error
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

func (r *notificationRepository) FindPage(userID uuid.UUID, limit, offset int) ([]model.Notification, error) {
	var notifications []model.Notification
	err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	return notifications, err
}

func (r *notificationRepository) CountAll(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *notificationRepository) CountUnread(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Count(&count).Error
	return count, err
}

func (r *notificationRepository) MarkAsRead(id uuid.UUID, userID uuid.UUID) (int64, error) {
	result := r.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true)
	return result.RowsAffected, result.Error
}

func (r *notificationRepository) MarkAllAsRead(userID uuid.UUID) error {
	return r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Update("is_read", true).Error
}

func (r *notificationRepository) Delete(id uuid.UUID, userID uuid.UUID) (int64, error) {
	result := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Notification{})
	return result.RowsAffected, result.Error
}

func (r *notificationRepository) DeleteAllByUserID(userID uuid.UUID) error {
	return r.db.
		Where("user_id = ?", userID).
		Delete(&model.Notification{}).Error
}
