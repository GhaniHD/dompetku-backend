package handler

import (
	"dompetku/pkg/utils"
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TransferHandler struct {
	transferService service.TransferService
}

func NewTransferHandler(transferService service.TransferService) *TransferHandler {
	return &TransferHandler{transferService: transferService}
}

func (h *TransferHandler) RegisterRoutes(rg *gin.RouterGroup) {
	transfers := rg.Group("/transfers")
	{
		transfers.POST("", h.Create)
		transfers.GET("", h.GetAll)
		transfers.GET("/:id", h.GetByID)
		transfers.DELETE("/:id", h.Delete)
	}
}

// POST /api/transfers
func (h *TransferHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.transferService.Create(userID, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusCreated, "transfer berhasil dibuat", result)
}

// GET /api/transfers?wallet_id=&start_date=&end_date=
func (h *TransferHandler) GetAll(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var filter dto.TransferFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	filter.StartDate, filter.EndDate = utils.CalendarDate(filter.StartDate), utils.CalendarDate(filter.EndDate)

	results, err := h.transferService.GetAll(userID, filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "data transfer berhasil diambil", results)
}

// GET /api/transfers/:id
func (h *TransferHandler) GetByID(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID transfer tidak valid")
		return
	}

	result, err := h.transferService.GetByID(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "detail transfer berhasil diambil", result)
}

// DELETE /api/transfers/:id
func (h *TransferHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID transfer tidak valid")
		return
	}

	if err := h.transferService.Delete(userID, id); err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "transfer berhasil dihapus", nil)
}
