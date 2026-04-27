package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware mengkonfigurasi Cross-Origin Resource Sharing
// Mengembalikan gin.HandlerFunc sebagai pengganti *cors.Cors dari rs/cors
func CORSMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		// Daftar origin yang diizinkan mengakses API
		AllowOrigins: []string{
			"http://localhost:5173",
			"https://dompetku-new.vercel.app",
		},

		// Method HTTP yang diizinkan
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},

		// Header yang diizinkan dikirim oleh client
		AllowHeaders: []string{"Authorization", "Content-Type"},

		// Izinkan pengiriman cookie / credentials
		AllowCredentials: true,
	})
}