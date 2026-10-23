package handler

import (
	"dompetku/internal/dto"
	"dompetku/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type NotificationHandler struct {
	svc service.NotificationService
}

func NewNotificationHandler(svc service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) RegisterRoutes(r *gin.RouterGroup) {
	notif := r.Group("/notifications")
	{
		notif.GET("", h.GetAll)
		notif.PATCH("/read-all", h.MarkAllAsRead)
		notif.PATCH("/:id/read", h.MarkAsRead)
		notif.DELETE("/clear", h.DeleteAll)
		notif.DELETE("/:id", h.Delete)
	}
}

// GET /api/notifications?page=1&limit=20
func (h *NotificationHandler) GetAll(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var filter dto.NotificationFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.svc.GetAll(userID, filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "berhasil mengambil notifikasi", result)
}

// PATCH /api/notifications/:id/read
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "id tidak valid")
		return
	}

	if err := h.svc.MarkAsRead(id, userID); err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "notifikasi ditandai sudah dibaca", nil)
}

// PATCH /api/notifications/read-all
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	if err := h.svc.MarkAllAsRead(userID); err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "semua notifikasi ditandai sudah dibaca", nil)
}

// DELETE /api/notifications/:id
func (h *NotificationHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "id tidak valid")
		return
	}

	if err := h.svc.Delete(id, userID); err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "notifikasi dihapus", nil)
}

// DELETE /api/notifications/clear
func (h *NotificationHandler) DeleteAll(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	if err := h.svc.DeleteAll(userID); err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "semua notifikasi dihapus", nil)
}
