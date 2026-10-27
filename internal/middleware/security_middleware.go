package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"dompetku/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders memasang header dasar untuk semua respons API dan file upload.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		if strings.HasPrefix(c.Request.URL.Path, "/uploads/") {
			h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
			h.Set("Cross-Origin-Resource-Policy", "cross-origin")
		}
		c.Next()
	}
}

// BodyLimit membatasi ukuran body request; upload avatar memakai batas sendiri.
func BodyLimit(defaultBytes, uploadBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := defaultBytes
		if strings.HasSuffix(c.Request.URL.Path, "/avatar") {
			limit = uploadBytes
		}
		if c.Request.ContentLength > limit {
			abortJSON(c, http.StatusRequestEntityTooLarge, "ukuran data terlalu besar")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

func TooManyRequests(c *gin.Context, retry time.Duration) {
	seconds := int(math.Ceil(retry.Seconds()))
	c.Header("Retry-After", fmt.Sprint(seconds))
	wait := fmt.Sprintf("%d detik", seconds)
	if seconds >= 60 {
		wait = fmt.Sprintf("%d menit", int(math.Ceil(float64(seconds)/60)))
	}
	abortJSON(c, http.StatusTooManyRequests, "Terlalu banyak percobaan. Coba lagi dalam "+wait+".")
}

// RateLimitIP membatasi request per alamat IP untuk satu kelompok endpoint.
func RateLimitIP(l *ratelimit.Limiter, name string, limit int, per time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ok, retry := l.Allow(name+":"+c.ClientIP(), limit, per); !ok {
			TooManyRequests(c, retry)
			return
		}
		c.Next()
	}
}

// RateLimitUser wajib dipasang setelah AuthMiddleware.
func RateLimitUser(l *ratelimit.Limiter, limit int, per time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := c.Get("user_id")
		if ok, retry := l.Allow(fmt.Sprint("user:", id), limit, per); !ok {
			TooManyRequests(c, retry)
			return
		}
		c.Next()
	}
}
