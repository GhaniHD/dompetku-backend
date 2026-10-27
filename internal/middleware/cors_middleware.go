package middleware

import (
	"dompetku/pkg/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware mengkonfigurasi Cross-Origin Resource Sharing
// Mengembalikan gin.HandlerFunc sebagai pengganti *cors.Cors dari rs/cors
func CORSMiddleware(cfg *config.AppConf) gin.HandlerFunc {
	return cors.New(cors.Config{
		// Daftar origin yang diizinkan mengakses API
		AllowOrigins: cfg.CORSOrigins,

		// Method HTTP yang diizinkan
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},

		// Header yang diizinkan dikirim oleh client
		AllowHeaders: []string{"Authorization", "Content-Type"},

		// Agar frontend bisa membaca nama file unduhan (backup).
		ExposeHeaders: []string{"Content-Disposition"},

		// Izinkan pengiriman cookie / credentials
		AllowCredentials: true,
	})
}
