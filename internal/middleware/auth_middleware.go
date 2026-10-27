package middleware

import (
	"dompetku/internal/model"
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

		user, err := userRepo.FindByID(c.Request.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				abortJSON(c, http.StatusUnauthorized, "akun tidak ditemukan, silakan login ulang")
				return
			}
			log.Printf("[auth] gagal cek user %s: %v", claims.UserID, err)
			abortJSON(c, http.StatusInternalServerError, "terjadi kesalahan internal")
			return
		}

		// Versi token naik saat login di perangkat lain, password diganti/direset,
		// atau akun dinonaktifkan, sehingga sesi lama langsung tidak berlaku.
		if claims.Version != user.TokenVersion {
			code, message := sessionEnded(user.SessionReason)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": code, "message": message, "data": nil})
			return
		}
		if user.Status != model.UserActive {
			abortJSON(c, http.StatusUnauthorized, statusMessage(user.Status))
			return
		}

		c.Set("role", user.Role)
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

func sessionEnded(reason string) (string, string) {
	switch reason {
	case model.SessionLogin:
		return "session_replaced", "Kamu keluar karena akun ini masuk di perangkat lain."
	case model.SessionPassword:
		return "password_changed", "Password akun ini baru diganti. Silakan masuk lagi."
	default:
		return "session_ended", "Sesi berakhir, silakan masuk lagi."
	}
}

func statusMessage(status string) string {
	switch status {
	case model.UserPending:
		return "akun kamu menunggu persetujuan admin"
	case model.UserSuspended:
		return "akun ini dinonaktifkan admin"
	default:
		return "akun ini tidak aktif"
	}
}

// RequireAdmin wajib dipasang setelah AuthMiddleware.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, _ := c.Get("role"); role != model.RoleAdmin {
			abortJSON(c, http.StatusForbidden, "hanya admin yang boleh mengakses")
			return
		}
		c.Next()
	}
}
