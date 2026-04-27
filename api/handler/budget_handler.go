package handler

import (
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

type BudgetHandler struct {
	budgetService service.BudgetService
}

type BudgetHandlerParams struct {
	fx.In
	BudgetService service.BudgetService
}

func NewBudgetHandler(p BudgetHandlerParams) *BudgetHandler {
	return &BudgetHandler{budgetService: p.BudgetService}
}

func (h *BudgetHandler) RegisterRoutes(rg *gin.RouterGroup) {
	budgets := rg.Group("/budgets")
	{
		budgets.POST("", h.CreateBudget)
		budgets.GET("", h.GetAllBudgets)
		budgets.GET("/:id", h.GetBudgetByID)
		budgets.PUT("/:id", h.UpdateBudget)
		budgets.DELETE("/:id", h.DeleteBudget)
		budgets.POST("/copy", h.CopyBudget)
	}
}

// POST /api/budgets
func (h *BudgetHandler) CreateBudget(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.CreateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.budgetService.CreateBudget(userID, req)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	successResponse(c, http.StatusCreated, "anggaran berhasil dibuat", result)
}

// GET /api/budgets?month=6&year=2025
func (h *BudgetHandler) GetAllBudgets(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var filter dto.BudgetFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.budgetService.GetAllBudgets(userID, filter)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "data anggaran berhasil diambil", result)
}

// GET /api/budgets/:id
func (h *BudgetHandler) GetBudgetByID(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID anggaran tidak valid")
		return
	}

	result, err := h.budgetService.GetBudgetByID(userID, id)
	if err != nil {
		if err.Error() == "anggaran tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "detail anggaran berhasil diambil", result)
}

// PUT /api/budgets/:id
func (h *BudgetHandler) UpdateBudget(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID anggaran tidak valid")
		return
	}

	var req dto.UpdateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.budgetService.UpdateBudget(userID, id, req)
	if err != nil {
		if err.Error() == "anggaran tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "anggaran berhasil diperbarui", result)
}

// DELETE /api/budgets/:id
func (h *BudgetHandler) DeleteBudget(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID anggaran tidak valid")
		return
	}

	if err := h.budgetService.DeleteBudget(userID, id); err != nil {
		if err.Error() == "anggaran tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "anggaran berhasil dihapus", nil)
}

// POST /api/budgets/copy
func (h *BudgetHandler) CopyBudget(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.CopyBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	results, err := h.budgetService.CopyBudget(userID, req)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	successResponse(c, http.StatusCreated, "anggaran berhasil disalin", results)
}