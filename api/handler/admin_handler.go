package handler

import (
	"net/http"
	"strconv"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AdminHandler struct {
	svc    service.AdminService
	emails service.EmailService
}

func NewAdminHandler(svc service.AdminService, emails service.EmailService) *AdminHandler {
	return &AdminHandler{svc: svc, emails: emails}
}

func (h *AdminHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/summary", h.Summary)
	rg.GET("/users", h.ListUsers)
	rg.POST("/users/:id/approve", h.userAction(func(admin, user uuid.UUID, reason string) error { return h.svc.Approve(admin, user) }, "pengguna disetujui"))
	rg.POST("/users/:id/reject", h.userAction(h.svc.Reject, "pendaftaran ditolak"))
	rg.POST("/users/:id/suspend", h.userAction(h.svc.Suspend, "akun dinonaktifkan"))
	rg.POST("/users/:id/activate", h.userAction(func(admin, user uuid.UUID, reason string) error { return h.svc.Activate(admin, user) }, "akun diaktifkan lagi"))
	rg.GET("/users/:id", h.UserDetail)
	rg.POST("/users/:id/reset-link", h.ResetLink)
	rg.POST("/users/:id/logout", h.confirmed(func(c *gin.Context, admin, user uuid.UUID) error {
		return h.svc.ForceLogout(admin, user, clientInfo(c))
	}, "pengguna dikeluarkan dari semua perangkat"))
	rg.DELETE("/users/:id", h.confirmed(func(c *gin.Context, admin, user uuid.UUID) error { return h.svc.Delete(admin, user) }, "akun dihapus"))
	rg.GET("/activity", h.Activity)
	rg.GET("/announcements", h.ListAnnouncements)
	rg.POST("/announcements", h.CreateAnnouncement)
	rg.POST("/announcements/:id/end", h.EndAnnouncement)
	rg.GET("/invites", h.ListInvites)
	rg.POST("/invites", h.CreateInvite)
	rg.DELETE("/invites/:id", h.RevokeInvite)
	rg.GET("/logs", h.Logs)
	rg.POST("/users/:id/verify-email", h.confirmed(func(c *gin.Context, admin, user uuid.UUID) error { return h.emails.MarkVerified(admin, user) }, "email ditandai terverifikasi"))
	rg.GET("/email-stats", h.EmailStats)
}

// RegisterPublic: endpoint untuk semua pengguna yang sudah login.
func (h *AdminHandler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/announcements", h.ActiveAnnouncements)
}

// checkPassword meminta admin mengetik ulang password untuk aksi berbahaya.
func (h *AdminHandler) checkPassword(c *gin.Context, admin uuid.UUID) bool {
	var req dto.AdminPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "masukkan password admin untuk melanjutkan")
		return false
	}
	if err := h.svc.VerifyPassword(admin, req.Password); err != nil {
		errorResponse(c, http.StatusForbidden, err.Error())
		return false
	}
	return true
}

func (h *AdminHandler) confirmed(fn func(c *gin.Context, admin, user uuid.UUID) error, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, target, ok := h.ids(c)
		if !ok || !h.checkPassword(c, admin) {
			return
		}
		if err := fn(c, admin, target); err != nil {
			serviceErrorResponse(c, err)
			return
		}
		successResponse(c, http.StatusOK, message, nil)
	}
}

func (h *AdminHandler) UserDetail(c *gin.Context) {
	_, target, ok := h.ids(c)
	if !ok {
		return
	}
	result, err := h.svc.UserDetail(target)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "detail pengguna", result)
}

func (h *AdminHandler) Activity(c *gin.Context) {
	var filter dto.AuthEventFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, "filter tidak valid")
		return
	}
	result, err := h.svc.AuthEvents(filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "aktivitas akun", result)
}

func (h *AdminHandler) ListAnnouncements(c *gin.Context) {
	result, err := h.svc.ListAnnouncements()
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "daftar pengumuman", result)
}

func (h *AdminHandler) ActiveAnnouncements(c *gin.Context) {
	result, err := h.svc.ActiveAnnouncements()
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "pengumuman aktif", result)
}

func (h *AdminHandler) CreateAnnouncement(c *gin.Context) {
	admin, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.AnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "isi pengumuman 1–280 karakter, jenis info atau warning")
		return
	}
	result, err := h.svc.CreateAnnouncement(admin, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusCreated, "pengumuman dipasang", result)
}

func (h *AdminHandler) EndAnnouncement(c *gin.Context) {
	admin, target, ok := h.ids(c)
	if !ok {
		return
	}
	if err := h.svc.EndAnnouncement(admin, target); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "pengumuman diakhiri", nil)
}

func (h *AdminHandler) ids(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	admin, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return uuid.Nil, uuid.Nil, false
	}
	target, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "ID tidak valid")
		return uuid.Nil, uuid.Nil, false
	}
	return admin, target, true
}

func (h *AdminHandler) userAction(fn func(admin, user uuid.UUID, reason string) error, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, target, ok := h.ids(c)
		if !ok {
			return
		}
		var req dto.AdminReasonRequest
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				errorResponse(c, http.StatusBadRequest, "alasan maksimal 300 karakter")
				return
			}
		}
		if err := fn(admin, target, req.Reason); err != nil {
			serviceErrorResponse(c, err)
			return
		}
		successResponse(c, http.StatusOK, message, nil)
	}
}

func (h *AdminHandler) Summary(c *gin.Context) {
	result, err := h.svc.Summary()
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "ringkasan admin", result)
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var filter dto.AdminUserFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, "filter tidak valid")
		return
	}
	result, err := h.svc.ListUsers(filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	if result == nil {
		result = []dto.AdminUserResponse{}
	}
	successResponse(c, http.StatusOK, "daftar pengguna", result)
}

func (h *AdminHandler) ResetLink(c *gin.Context) {
	admin, target, ok := h.ids(c)
	if !ok || !h.checkPassword(c, admin) {
		return
	}
	result, err := h.svc.ResetLink(admin, target)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusCreated, "link atur ulang password dibuat", result)
}

func (h *AdminHandler) ListInvites(c *gin.Context) {
	result, err := h.svc.ListInvites()
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "daftar undangan", result)
}

func (h *AdminHandler) CreateInvite(c *gin.Context) {
	admin, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.CreateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "batas pemakaian 1–50 dan masa berlaku 1–30 hari")
		return
	}
	result, err := h.svc.CreateInvite(admin, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusCreated, "undangan dibuat", result)
}

func (h *AdminHandler) RevokeInvite(c *gin.Context) {
	admin, target, ok := h.ids(c)
	if !ok {
		return
	}
	if err := h.svc.RevokeInvite(admin, target); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "undangan dibatalkan", nil)
}

func (h *AdminHandler) Logs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	result, err := h.svc.Logs(limit)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	if result == nil {
		result = []dto.AdminLogResponse{}
	}
	successResponse(c, http.StatusOK, "log admin", result)
}

func (h *AdminHandler) EmailStats(c *gin.Context) {
	result, err := h.emails.Stats()
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "statistik email", result)
}
