package handler

import (
	"errors"
	"log"
	"strings"

	"dompetku/internal/dto"
	"dompetku/internal/middleware"
	"dompetku/internal/service"
	"dompetku/pkg/ratelimit"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthHandler menangani request HTTP yang berkaitan dengan autentikasi
type AuthHandler struct {
	authService service.AuthService
	limiter     *ratelimit.Limiter
}

// NewAuthHandler membuat instance baru AuthHandler
func NewAuthHandler(authService service.AuthService, limiter *ratelimit.Limiter) *AuthHandler {
	return &AuthHandler{authService: authService, limiter: limiter}
}

const (
	maxLoginFailures = 5
	loginLockout     = 15 * time.Minute
)

func clientInfo(c *gin.Context) dto.ClientInfo {
	return dto.ClientInfo{UserAgent: c.Request.UserAgent(), IP: c.ClientIP()}
}

// Register godoc
// @Summary      Daftar akun baru
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.RegisterRequest true "Data registrasi"
// @Success      201
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest

	// Bind dan validasi request body
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "format request tidak valid")
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), req, clientInfo(c))
	if err != nil {
		if !isUserFacing(err) {
			serviceErrorResponse(c, err)
			return
		}
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	message := "registrasi berhasil"
	if resp.VerificationRequired {
		message = "akun dibuat, masukkan kode verifikasi yang dikirim ke emailmu"
	} else if resp.Token == "" {
		message = "pendaftaran terkirim, tunggu persetujuan admin"
	}
	successResponse(c, http.StatusCreated, message, resp)
}

// Login godoc
// @Summary      Login akun
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.LoginRequest true "Data login"
// @Success      200
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest

	// Bind dan validasi request body
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "format request tidak valid")
		return
	}

	// Kunci per email setelah beberapa kali gagal, terlepas dari IP penebak.
	failKey := "login-fail:" + strings.ToLower(strings.TrimSpace(req.Email))
	if blocked, retry := h.limiter.Blocked(failKey, maxLoginFailures, loginLockout); blocked {
		h.authService.RecordLoginLocked(req.Email, clientInfo(c))
		middleware.TooManyRequests(c, retry)
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req, clientInfo(c))
	if errors.Is(err, service.ErrInvalidCredentials) {
		h.limiter.Allow(failKey, maxLoginFailures, loginLockout)
	}
	if err == nil {
		h.limiter.Reset(failKey)
	}
	if err != nil {
		if emailErrorResponse(c, err) {
			return
		}
		errorResponse(c, http.StatusUnauthorized, err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /api/auth/invite/:code
func (h *AuthHandler) CheckInvite(c *gin.Context) {
	result, err := h.authService.CheckInvite(c.Param("code"))
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "status undangan", result)
}

// POST /api/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "token dan password baru wajib diisi")
		return
	}
	if err := h.authService.ResetPassword(req, clientInfo(c)); err != nil {
		if errors.Is(err, service.ErrResetTokenInvalid) {
			errorResponse(c, http.StatusBadRequest, err.Error())
			return
		}
		if !strings.HasPrefix(err.Error(), "password ") {
			serviceErrorResponse(c, err)
			return
		}
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	successResponse(c, http.StatusOK, "password berhasil diatur ulang, silakan login", nil)
}

// isUserFacing membedakan pesan validasi pendaftaran dari error database.
func isUserFacing(err error) bool {
	if errors.Is(err, service.ErrInviteInvalid) {
		return true
	}
	msg := err.Error()
	for _, prefix := range []string{"nama, email", "format email", "password harus", "email sudah"} {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	if err := h.authService.Logout(c.Request.Context(), userID, clientInfo(c)); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "berhasil keluar", nil)
}

// POST /api/auth/verify — masukkan kode verifikasi email saat login.
func (h *AuthHandler) VerifyLogin(c *gin.Context) {
	var req dto.VerifyLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "kode verifikasi wajib diisi")
		return
	}
	resp, err := h.authService.VerifyLogin(c.Request.Context(), req, clientInfo(c))
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// POST /api/auth/verify/send — kirim ulang kode verifikasi saat login.
func (h *AuthHandler) SendLoginCode(c *gin.Context) {
	var req struct {
		VerifyToken string `json:"verify_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "format request tidak valid")
		return
	}
	resp, err := h.authService.SendLoginCode(req.VerifyToken)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "kode verifikasi dikirim", resp)
}

// POST /api/auth/forgot-password — jawaban selalu sama agar email terdaftar tidak bisa ditebak.
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "email wajib diisi")
		return
	}
	if err := h.authService.ForgotPassword(req.Email); err != nil {
		if errors.Is(err, service.ErrMailDisabled) {
			serviceErrorResponse(c, err)
			return
		}
		log.Printf("[auth] lupa password: %v", err)
	}
	successResponse(c, http.StatusOK, "kalau email itu terdaftar, link atur ulang password sudah dikirim", nil)
}

// GET /api/auth/features
func (h *AuthHandler) Features(c *gin.Context) {
	successResponse(c, http.StatusOK, "fitur", h.authService.Features())
}
