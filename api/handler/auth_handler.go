package handler

import (
	"dompetku/internal/dto"
	"dompetku/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthHandler menangani request HTTP yang berkaitan dengan autentikasi
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler membuat instance baru AuthHandler
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
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

	resp, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	successResponse(c, http.StatusCreated, "registrasi berhasil", resp)
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

	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		errorResponse(c, http.StatusUnauthorized, err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}