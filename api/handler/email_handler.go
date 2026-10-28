package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/middleware"
	"dompetku/internal/service"
	"dompetku/pkg/ratelimit"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EmailHandler struct {
	emails  service.EmailService
	profile service.ProfileService
	limiter *ratelimit.Limiter
}

func NewEmailHandler(emails service.EmailService, profile service.ProfileService, limiter *ratelimit.Limiter) *EmailHandler {
	return &EmailHandler{emails: emails, profile: profile, limiter: limiter}
}

// RegisterPublic: endpoint tanpa login (berhenti langganan dan webhook Brevo).
func (h *EmailHandler) RegisterPublic(rg *gin.RouterGroup) {
	rg.POST("/unsubscribe", middleware.RateLimitIP(h.limiter, "unsubscribe", 30, 15*time.Minute), h.Unsubscribe)
	rg.POST("/webhook/brevo", middleware.RateLimitIP(h.limiter, "brevo", 600, time.Minute), h.BrevoWebhook)
}

func (h *EmailHandler) RegisterProfile(rg *gin.RouterGroup) {
	rg.POST("/profile/email", middleware.RateLimitUserNamed(h.limiter, "email-change", 10, time.Hour), h.ChangeEmail)
	rg.DELETE("/profile/email/pending", h.CancelChange)
	rg.POST("/profile/email/send-code", h.SendCode)
	rg.POST("/profile/email/verify", middleware.RateLimitUserNamed(h.limiter, "email-verify", 30, 15*time.Minute), h.VerifyCode)
	rg.PUT("/profile/email-prefs", h.UpdatePrefs)
}

func (h *EmailHandler) profileReply(c *gin.Context, userID uuid.UUID, message string) {
	resp, err := h.profile.GetProfile(c.Request.Context(), userID)
	if err != nil {
		profileErrResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, message, resp)
}

// POST /api/profile/email
func (h *EmailHandler) ChangeEmail(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "email baru dan password wajib diisi")
		return
	}
	sent, err := h.emails.StartEmailChange(c.Request.Context(), userID, req, clientInfo(c))
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	message := "kode verifikasi dikirim ke email baru"
	if !sent.Sent {
		message = "email berhasil diganti"
	}
	successResponse(c, http.StatusOK, message, sent)
}

// DELETE /api/profile/email/pending
func (h *EmailHandler) CancelChange(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	if err := h.emails.CancelEmailChange(userID); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	h.profileReply(c, userID, "perubahan email dibatalkan")
}

// POST /api/profile/email/send-code — untuk email baru bila ada, selain itu email sekarang.
func (h *EmailHandler) SendCode(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	sent, err := h.emails.SendCode(userID, "")
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "kode verifikasi dikirim", sent)
}

// POST /api/profile/email/verify
func (h *EmailHandler) VerifyCode(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "kode verifikasi wajib diisi")
		return
	}
	if _, err := h.emails.ConfirmCode(userID, "", req.Code, clientInfo(c)); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	h.profileReply(c, userID, "email terverifikasi")
}

// PUT /api/profile/email-prefs
func (h *EmailHandler) UpdatePrefs(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.EmailPrefs
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "format request tidak valid")
		return
	}
	if err := h.emails.UpdatePrefs(userID, req); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	h.profileReply(c, userID, "pengaturan email disimpan")
}

// POST /api/email/unsubscribe?token= — dipanggil halaman berhenti langganan
// atau tombol satu klik di aplikasi email (body form List-Unsubscribe=One-Click).
func (h *EmailHandler) Unsubscribe(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		var req dto.UnsubscribeRequest
		_ = c.ShouldBindJSON(&req)
		token = req.Token
	}
	resp, err := h.emails.Unsubscribe(token)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "kamu tidak akan menerima "+resp.Label+" lagi", resp)
}

// POST /api/email/webhook/brevo?secret= — menerima satu event atau daftar event.
func (h *EmailHandler) BrevoWebhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 256<<10))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "body tidak valid")
		return
	}
	var events []dto.BrevoEvent
	if err := json.Unmarshal(body, &events); err != nil {
		var one dto.BrevoEvent
		if err := json.Unmarshal(body, &one); err != nil {
			errorResponse(c, http.StatusBadRequest, "body tidak valid")
			return
		}
		events = []dto.BrevoEvent{one}
	}
	if err := h.emails.HandleBrevo(c.Query("secret"), events); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "ok", nil)
}
