package handler

import (
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

type ReportHandler struct {
	reportService service.ReportService
}

type ReportHandlerParams struct {
	fx.In
	ReportService service.ReportService
}

func NewReportHandler(p ReportHandlerParams) *ReportHandler {
	return &ReportHandler{reportService: p.ReportService}
}

func (h *ReportHandler) RegisterRoutes(rg *gin.RouterGroup) {
	reports := rg.Group("/reports")
	{
		reports.GET("", h.GetMonthlyReport)          // GET /api/reports?month=&year=
		reports.GET("/yearly", h.GetYearlyReport)    // GET /api/reports/yearly?year=
	}
}

// GET /api/reports?month=&year=
func (h *ReportHandler) GetMonthlyReport(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var filter dto.ReportFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, "parameter month dan year wajib diisi")
		return
	}
	result, err := h.reportService.GetMonthlyReport(userID, filter)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(c, http.StatusOK, "laporan bulanan berhasil diambil", result)
}

// GET /api/reports/yearly?year=
func (h *ReportHandler) GetYearlyReport(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var filter dto.YearlyReportFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, "parameter year wajib diisi")
		return
	}
	result, err := h.reportService.GetYearlyReport(userID, filter)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(c, http.StatusOK, "laporan tahunan berhasil diambil", result)
}