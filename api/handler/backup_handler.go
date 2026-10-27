package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"
	"dompetku/pkg/backupcrypt"

	"github.com/gin-gonic/gin"
)

// MaxBackupFileSize dipakai juga oleh middleware batas body di routes.go.
const MaxBackupFileSize = 20 << 20

type BackupHandler struct {
	svc service.BackupService
}

func NewBackupHandler(svc service.BackupService) *BackupHandler {
	return &BackupHandler{svc: svc}
}

func backupErrorResponse(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBackupAccountPassword):
		errorResponse(c, http.StatusForbidden, err.Error())
	case errors.Is(err, backupcrypt.ErrDecrypt),
		errors.Is(err, backupcrypt.ErrNotBackup),
		errors.Is(err, backupcrypt.ErrTooLarge),
		errors.Is(err, service.ErrBackupFormat),
		errors.Is(err, service.ErrBackupInvalid),
		errors.Is(err, service.ErrBackupConfirm),
		errors.Is(err, service.ErrBackupMismatch):
		errorResponse(c, http.StatusBadRequest, err.Error())
	default:
		serviceErrorResponse(c, err)
	}
}

// POST /api/backup/export  {password, backup_password}
func (h *BackupHandler) Export(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.BackupExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "password akun wajib diisi dan password backup minimal 8 karakter")
		return
	}
	data, filename, err := h.svc.Export(c.Request.Context(), userID, req)
	if err != nil {
		backupErrorResponse(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// readBackupForm membaca form multipart: file, password, backup_password.
func readBackupForm(c *gin.Context) (file []byte, password, backupPassword string, ok bool) {
	header, err := c.FormFile("file")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "pilih file backup (.dpk)")
		return nil, "", "", false
	}
	if header.Size > MaxBackupFileSize {
		errorResponse(c, http.StatusRequestEntityTooLarge, "file backup maksimal 20 MB")
		return nil, "", "", false
	}
	f, err := header.Open()
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "file backup tidak bisa dibaca")
		return nil, "", "", false
	}
	defer f.Close()
	file, err = io.ReadAll(io.LimitReader(f, MaxBackupFileSize))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "file backup tidak bisa dibaca")
		return nil, "", "", false
	}
	password, backupPassword = c.PostForm("password"), c.PostForm("backup_password")
	if password == "" || backupPassword == "" {
		errorResponse(c, http.StatusBadRequest, "password akun dan password backup wajib diisi")
		return nil, "", "", false
	}
	return file, password, backupPassword, true
}

// POST /api/backup/preview  (multipart: file, password, backup_password)
func (h *BackupHandler) Preview(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	file, password, backupPassword, ok := readBackupForm(c)
	if !ok {
		return
	}
	preview, err := h.svc.Preview(c.Request.Context(), userID, password, backupPassword, file)
	if err != nil {
		backupErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "isi backup", preview)
}

// POST /api/backup/restore  (multipart: file, password, backup_password, confirm)
func (h *BackupHandler) Restore(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	file, password, backupPassword, ok := readBackupForm(c)
	if !ok {
		return
	}
	summary, err := h.svc.Restore(c.Request.Context(), userID, password, backupPassword, c.PostForm("confirm"), file)
	if err != nil {
		backupErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "data berhasil dipulihkan dari backup", summary)
}
