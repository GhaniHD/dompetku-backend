package middleware

import (
	"dompetku/pkg/config"
	"dompetku/pkg/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware memvalidasi JWT token dari header Authorization
// dan menyimpan userID ke Gin context untuk digunakan oleh handler
func AuthMiddleware(cfg *config.AppConf) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "authorization header tidak ditemukan",
				"data":    nil,
			})
			c.Abort() // Hentikan request, jangan lanjut ke handler
			return
		}

		// Format header yang diharapkan: Bearer <token>
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "format authorization header tidak valid",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Validasi token JWT
		token := parts[1]
		claims, err := utils.ValidateToken(token, cfg.JWTSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "token tidak valid atau sudah kadaluarsa",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Simpan userID ke Gin context agar bisa diambil oleh handler
		// Key "user_id" harus sesuai dengan c.Get("user_id") di handler
		c.Set("user_id", claims.UserID)

		// Lanjutkan ke handler berikutnya
		c.Next()
	}
}