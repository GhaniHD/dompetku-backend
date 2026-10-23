package handler

import (
	"net/http"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

type TransactionHandler struct {
	transactionService service.TransactionService
}

type TransactionHandlerParams struct {
	fx.In
	TransactionService service.TransactionService
}

func NewTransactionHandler(p TransactionHandlerParams) *TransactionHandler {
	return &TransactionHandler{transactionService: p.TransactionService}
}

func (h *TransactionHandler) RegisterRoutes(rg *gin.RouterGroup) {
	transactions := rg.Group("/transactions")
	{
		transactions.POST("", h.CreateTransaction)
		transactions.GET("", h.GetAllTransactions)
		transactions.GET("/:id", h.GetTransactionByID)
		transactions.PUT("/:id", h.UpdateTransaction)
		transactions.DELETE("/:id", h.DeleteTransaction)
	}
}

// POST /api/transactions
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.transactionService.CreateTransaction(userID, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusCreated, "transaksi berhasil dibuat", result)
}

// GET /api/transactions
func (h *TransactionHandler) GetAllTransactions(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var filter dto.TransactionFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	results, err := h.transactionService.GetAllTransactions(userID, filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "data transaksi berhasil diambil", results)
}

// GET /api/transactions/:id
func (h *TransactionHandler) GetTransactionByID(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID transaksi tidak valid")
		return
	}

	result, err := h.transactionService.GetTransactionByID(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "detail transaksi berhasil diambil", result)
}

// PUT /api/transactions/:id
func (h *TransactionHandler) UpdateTransaction(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID transaksi tidak valid")
		return
	}

	var req dto.UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.transactionService.UpdateTransaction(userID, id, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "transaksi berhasil diperbarui", result)
}

// DELETE /api/transactions/:id
func (h *TransactionHandler) DeleteTransaction(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID transaksi tidak valid")
		return
	}

	if err := h.transactionService.DeleteTransaction(userID, id); err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "transaksi berhasil dihapus", nil)
}
