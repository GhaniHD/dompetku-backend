package handler

import (
	"dompetku/internal/service"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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

func successResponse(c *gin.Context, statusCode int, message string, data any) {
	c.JSON(statusCode, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

func errorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"success": false,
		"message": message,
		"data":    nil,
	})
}

func unauthorizedResponse(c *gin.Context) {
	errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
}

// serviceErrorResponse memetakan sentinel error service ke status HTTP. Error lain
// dicatat di log server dan tidak diteruskan ke client agar detail DB tidak bocor.
func serviceErrorResponse(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTransactionNotFound),
		errors.Is(err, service.ErrWalletNotFound),
		errors.Is(err, service.ErrCategoryNotFound):
		errorResponse(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrCategoryTypeMismatch),
		errors.Is(err, service.ErrBudgetCategoryNotExpense):
		errorResponse(c, http.StatusBadRequest, err.Error())
	default:
		log.Printf("[%s %s] %v", c.Request.Method, c.FullPath(), err)
		errorResponse(c, http.StatusInternalServerError, "terjadi kesalahan internal")
	}
}
