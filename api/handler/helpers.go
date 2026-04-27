// api/handler/helpers.go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// getUserID mengambil userID dari Gin context yang di-set oleh AuthMiddleware.
// Key "user_id" harus sesuai dengan c.Set("user_id", ...) di auth_middleware.go.
// Mendukung dua bentuk: uuid.UUID langsung atau string UUID.
func getUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, false
	}
	switch v := val.(type) {
	case uuid.UUID:
		return v, true
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, false
		}
		return id, true
	}
	return uuid.Nil, false
}

// successResponse mengembalikan response standar untuk request yang berhasil.
func successResponse(c *gin.Context, statusCode int, message string, data any) {
	c.JSON(statusCode, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// errorResponse mengembalikan response standar untuk request yang gagal.
func errorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"success": false,
		"message": message,
		"data":    nil,
	})
}

// unauthorizedResponse shortcut untuk 401.
func unauthorizedResponse(c *gin.Context) {
	errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
}