package handler

import (
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RecurringHandler struct {
	svc service.RecurringService
}

func NewRecurringHandler(svc service.RecurringService) *RecurringHandler {
	return &RecurringHandler{svc: svc}
}

func (h *RecurringHandler) RegisterRoutes(rg *gin.RouterGroup) {
	recurring := rg.Group("/recurring")
	{
		recurring.POST("", h.Create)
		recurring.GET("", h.GetAll)
		recurring.GET("/:id", h.GetByID)
		recurring.PUT("/:id", h.Update)
		recurring.DELETE("/:id", h.Delete)
	}
}

func recurringIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID transaksi berulang tidak valid")
		return uuid.Nil, false
	}
	return id, true
}

// POST /api/recurring
func (h *RecurringHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.CreateRecurringRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.svc.Create(userID, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusCreated, "transaksi berulang berhasil dibuat", result)
}

// GET /api/recurring
func (h *RecurringHandler) GetAll(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	results, err := h.svc.GetAll(userID)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "data transaksi berulang berhasil diambil", results)
}

// GET /api/recurring/:id
func (h *RecurringHandler) GetByID(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := recurringIDParam(c)
	if !ok {
		return
	}

	result, err := h.svc.GetByID(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "detail transaksi berulang berhasil diambil", result)
}

// PUT /api/recurring/:id
func (h *RecurringHandler) Update(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := recurringIDParam(c)
	if !ok {
		return
	}

	var req dto.UpdateRecurringRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.svc.Update(userID, id, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "transaksi berulang berhasil diperbarui", result)
}

// DELETE /api/recurring/:id
func (h *RecurringHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := recurringIDParam(c)
	if !ok {
		return
	}

	if err := h.svc.Delete(userID, id); err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "transaksi berulang dihapus; transaksi yang sudah tercatat tetap ada", nil)
}
