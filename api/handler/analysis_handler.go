// api/handler/analysis_handler.go
package handler

import (
	"dompetku/internal/service"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AnalysisHandler struct {
	svc service.AnalysisService
}

func NewAnalysisHandler(svc service.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{svc: svc}
}

func (h *AnalysisHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/analysis", h.GetAnalysis)
}

// GET /api/analysis?year=2025
func (h *AnalysisHandler) GetAnalysis(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	// Default year = tahun sekarang
	year := time.Now().Year()
	if y := c.Query("year"); y != "" {
		parsed, err := strconv.Atoi(y)
		if err != nil || parsed < 2000 {
			errorResponse(c, http.StatusBadRequest, "parameter year tidak valid")
			return
		}
		year = parsed
	}

	result, err := h.svc.GetAnalysis(userID, year)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "gagal mengambil data analisis: "+err.Error())
		return
	}

	successResponse(c, http.StatusOK, "berhasil mengambil analisis keuangan", result)
}