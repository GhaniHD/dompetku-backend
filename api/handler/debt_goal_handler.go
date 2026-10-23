package handler

import (
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DebtGoalHandler struct {
	debts service.DebtService
	goals service.GoalService
}

func NewDebtGoalHandler(debts service.DebtService, goals service.GoalService) *DebtGoalHandler {
	return &DebtGoalHandler{debts: debts, goals: goals}
}

func (h *DebtGoalHandler) RegisterRoutes(rg *gin.RouterGroup) {
	debts := rg.Group("/debts")
	{
		debts.POST("", h.CreateDebt)
		debts.GET("", h.ListDebts)
		debts.GET("/:id", h.GetDebt)
		debts.PUT("/:id", h.UpdateDebt)
		debts.DELETE("/:id", h.DeleteDebt)
		debts.POST("/:id/payments", h.PayDebt)
		debts.DELETE("/:id/payments/:paymentId", h.DeleteDebtPayment)
	}

	goals := rg.Group("/goals")
	{
		goals.POST("", h.CreateGoal)
		goals.GET("", h.ListGoals)
		goals.GET("/:id", h.GetGoal)
		goals.PUT("/:id", h.UpdateGoal)
		goals.DELETE("/:id", h.DeleteGoal)
	}
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID tidak valid")
		return uuid.Nil, false
	}
	return id, true
}

// POST /api/debts
func (h *DebtGoalHandler) CreateDebt(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.CreateDebtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.debts.Create(userID, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusCreated, "hutang/piutang berhasil dicatat", result)
}

// GET /api/debts?kind=payable|receivable&status=open|paid
func (h *DebtGoalHandler) ListDebts(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var filter dto.DebtFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.debts.GetAll(userID, filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "data hutang/piutang berhasil diambil", result)
}

// GET /api/debts/:id
func (h *DebtGoalHandler) GetDebt(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	result, err := h.debts.GetByID(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "detail hutang/piutang berhasil diambil", result)
}

// PUT /api/debts/:id
func (h *DebtGoalHandler) UpdateDebt(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateDebtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.debts.Update(userID, id, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "hutang/piutang berhasil diperbarui", result)
}

// DELETE /api/debts/:id
func (h *DebtGoalHandler) DeleteDebt(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	if err := h.debts.Delete(userID, id); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "hutang/piutang dihapus dan saldo dompet dikembalikan", nil)
}

// POST /api/debts/:id/payments
func (h *DebtGoalHandler) PayDebt(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var req dto.DebtPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.debts.Pay(userID, id, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusCreated, "pembayaran berhasil dicatat", result)
}

// DELETE /api/debts/:id/payments/:paymentId
func (h *DebtGoalHandler) DeleteDebtPayment(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	paymentID, ok := uuidParam(c, "paymentId")
	if !ok {
		return
	}
	result, err := h.debts.DeletePayment(userID, id, paymentID)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "pembayaran dibatalkan", result)
}

// POST /api/goals
func (h *DebtGoalHandler) CreateGoal(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	var req dto.CreateGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.goals.Create(userID, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusCreated, "target tabungan berhasil dibuat", result)
}

// GET /api/goals
func (h *DebtGoalHandler) ListGoals(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	result, err := h.goals.GetAll(userID)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "data target tabungan berhasil diambil", result)
}

// GET /api/goals/:id
func (h *DebtGoalHandler) GetGoal(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	result, err := h.goals.GetByID(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "detail target tabungan berhasil diambil", result)
}

// PUT /api/goals/:id
func (h *DebtGoalHandler) UpdateGoal(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.goals.Update(userID, id, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "target tabungan berhasil diperbarui", result)
}

// DELETE /api/goals/:id
func (h *DebtGoalHandler) DeleteGoal(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	if err := h.goals.Delete(userID, id); err != nil {
		serviceErrorResponse(c, err)
		return
	}
	successResponse(c, http.StatusOK, "target tabungan dihapus", nil)
}
