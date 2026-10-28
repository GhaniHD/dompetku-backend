package handler

import (
	"dompetku/internal/service"
	"errors"
	"log"
	"net/http"
	"strconv"

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

func codedResponse(c *gin.Context, statusCode int, code, message string, data any) {
	c.JSON(statusCode, gin.H{
		"success": false,
		"code":    code,
		"message": message,
		"data":    data,
	})
}

// emailErrorResponse menangani error fitur email; false bila bukan error email.
func emailErrorResponse(c *gin.Context, err error) bool {
	var verify *service.VerificationRequiredError
	var cooldown *service.CooldownError
	var addr *service.AddressError
	switch {
	case errors.As(err, &verify):
		codedResponse(c, http.StatusForbidden, "email_unverified", err.Error(), gin.H{
			"verify_token": verify.VerifyToken, "email": verify.Email, "resend_in": verify.ResendIn,
		})
	case errors.As(err, &cooldown):
		c.Header("Retry-After", strconv.Itoa(cooldown.RetryAfter))
		codedResponse(c, http.StatusTooManyRequests, "code_cooldown", err.Error(), gin.H{"retry_after": cooldown.RetryAfter})
	case errors.As(err, &addr):
		codedResponse(c, http.StatusBadRequest, "email_address", err.Error(), nil)
	case errors.Is(err, service.ErrChallengeInvalid):
		codedResponse(c, http.StatusBadRequest, "challenge_expired", err.Error(), nil)
	case errors.Is(err, service.ErrCodeWrong), errors.Is(err, service.ErrCodeExpired), errors.Is(err, service.ErrCodeLocked):
		codedResponse(c, http.StatusBadRequest, "code_invalid", err.Error(), nil)
	case errors.Is(err, service.ErrMailDisabled):
		codedResponse(c, http.StatusServiceUnavailable, "mail_disabled", err.Error(), nil)
	case errors.Is(err, service.ErrAlreadyVerified), errors.Is(err, service.ErrEmailAlreadyUsed):
		errorResponse(c, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrEmailSame), errors.Is(err, service.ErrNoPendingEmail),
		errors.Is(err, service.ErrEmailReserved), errors.Is(err, service.ErrUnsubscribeInvalid):
		errorResponse(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrEmailPassword):
		errorResponse(c, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrWebhookSecret):
		errorResponse(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrAccountPending), errors.Is(err, service.ErrAccountRejected), errors.Is(err, service.ErrAccountSuspended):
		errorResponse(c, http.StatusForbidden, err.Error())
	default:
		return false
	}
	return true
}

// serviceErrorResponse memetakan sentinel error service ke status HTTP. Error lain
// dicatat di log server dan tidak diteruskan ke client agar detail DB tidak bocor.
func serviceErrorResponse(c *gin.Context, err error) {
	if emailErrorResponse(c, err) {
		return
	}
	switch {
	case errors.Is(err, service.ErrTransactionNotFound),
		errors.Is(err, service.ErrWalletNotFound),
		errors.Is(err, service.ErrCategoryNotFound),
		errors.Is(err, service.ErrTransferNotFound),
		errors.Is(err, service.ErrNotificationNotFound),
		errors.Is(err, service.ErrRecurringNotFound),
		errors.Is(err, service.ErrDebtNotFound),
		errors.Is(err, service.ErrDebtPaymentNotFound),
		errors.Is(err, service.ErrGoalNotFound),
		errors.Is(err, service.ErrAdminUserNotFound),
		errors.Is(err, service.ErrInviteNotFound),
		errors.Is(err, service.ErrAnnouncementGone):
		errorResponse(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrCategoryTypeMismatch),
		errors.Is(err, service.ErrBudgetCategoryNotExpense),
		errors.Is(err, service.ErrTransferSameWallet),
		errors.Is(err, service.ErrInsufficientBalance),
		errors.Is(err, service.ErrWalletArchived),
		errors.Is(err, service.ErrNegativeOpeningBalance),
		errors.Is(err, service.ErrWalletTypeNegative),
		errors.Is(err, service.ErrAdjustmentNoChange),
		errors.Is(err, service.ErrNoOpeningBalance),
		errors.Is(err, service.ErrRecurringEndBeforeStart),
		errors.Is(err, service.ErrDebtOverpaid),
		errors.Is(err, service.ErrAnnouncementEnds):
		errorResponse(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrTransactionIsTransferFee),
		errors.Is(err, service.ErrWalletNotEmpty),
		errors.Is(err, service.ErrGoalWalletTaken),
		errors.Is(err, service.ErrAdminInvalidStatus):
		errorResponse(c, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrAdminProtected):
		errorResponse(c, http.StatusForbidden, err.Error())
	default:
		log.Printf("[%s %s] %v", c.Request.Method, c.FullPath(), err)
		errorResponse(c, http.StatusInternalServerError, "terjadi kesalahan internal")
	}
}
