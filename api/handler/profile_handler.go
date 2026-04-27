// api/handler/profile_handler.go
package handler

import (
	"errors"
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	profileService service.ProfileService
}

func NewProfileHandler(profileService service.ProfileService) *ProfileHandler {
	return &ProfileHandler{profileService: profileService}
}

// RegisterRoutes mendaftarkan semua route profil ke router group yang sudah
// melewati AuthMiddleware (route group /api).
func (h *ProfileHandler) RegisterRoutes(rg *gin.RouterGroup) {
	profile := rg.Group("/profile")
	{
		profile.GET("", h.GetProfile)
		profile.PUT("", h.UpdateProfile)
		profile.PUT("/password", h.ChangePassword)
		profile.POST("/avatar", h.UploadAvatar)
		profile.DELETE("", h.DeleteAccount)
	}
}

// profileErrResponse memetakan sentinel error service ke HTTP status yang sesuai.
func profileErrResponse(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		errorResponse(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrEmailAlreadyUsed):
		errorResponse(c, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrWrongPassword):
		errorResponse(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrInvalidFileType):
		errorResponse(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrFileTooLarge):
		errorResponse(c, http.StatusRequestEntityTooLarge, err.Error())
	default:
		errorResponse(c, http.StatusInternalServerError, "terjadi kesalahan internal")
	}
}

// GET /api/profile
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	userID, ok := getUserID(c) // dari helpers.go
	if !ok {
		unauthorizedResponse(c)
		return
	}

	resp, err := h.profileService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		profileErrResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "berhasil", resp)
}

// PUT /api/profile
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.profileService.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		profileErrResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "profil berhasil diperbarui", resp)
}

// PUT /api/profile/password
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.profileService.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		profileErrResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "password berhasil diubah", nil)
}

// POST /api/profile/avatar
func (h *ProfileHandler) UploadAvatar(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "file avatar tidak ditemukan dalam request")
		return
	}

	resp, err := h.profileService.UploadAvatar(c.Request.Context(), userID, file)
	if err != nil {
		profileErrResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "avatar berhasil diperbarui", resp)
}

// DELETE /api/profile
func (h *ProfileHandler) DeleteAccount(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	if err := h.profileService.DeleteAccount(c.Request.Context(), userID); err != nil {
		profileErrResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "akun berhasil dihapus", nil)
}