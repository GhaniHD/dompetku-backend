package middleware

import (
	"dompetku/internal/repository"
	"dompetku/pkg/config"
	"dompetku/pkg/utils"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func abortJSON(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"success": false,
		"message": message,
		"data":    nil,
	})
}

// AuthMiddleware memvalidasi JWT dan memastikan akun pemilik token masih ada,
// sehingga token milik akun yang sudah dihapus langsung ditolak.
func AuthMiddleware(cfg *config.AppConf, userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortJSON(c, http.StatusUnauthorized, "authorization header tidak ditemukan")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			abortJSON(c, http.StatusUnauthorized, "format authorization header tidak valid")
			return
		}

		claims, err := utils.ValidateToken(parts[1], cfg.JWTSecret)
		if err != nil {
			abortJSON(c, http.StatusUnauthorized, "token tidak valid atau sudah kadaluarsa")
			return
		}

		if _, err := userRepo.FindByID(c.Request.Context(), claims.UserID); err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				abortJSON(c, http.StatusUnauthorized, "akun tidak ditemukan, silakan login ulang")
				return
			}
			log.Printf("[auth] gagal cek user %s: %v", claims.UserID, err)
			abortJSON(c, http.StatusInternalServerError, "terjadi kesalahan internal")
			return
		}

		c.Set("user_id", claims.UserID)
		c.Next()
	}
}
