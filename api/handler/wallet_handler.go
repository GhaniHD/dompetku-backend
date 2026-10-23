package handler

import (
	"dompetku/internal/dto"
	"dompetku/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

type WalletHandler struct {
	walletService service.WalletService
}

type WalletHandlerParams struct {
	fx.In
	WalletService service.WalletService
}

func NewWalletHandler(p WalletHandlerParams) *WalletHandler {
	return &WalletHandler{walletService: p.WalletService}
}

func (h *WalletHandler) RegisterRoutes(rg *gin.RouterGroup) {
	wallets := rg.Group("/wallets")
	{
		wallets.POST("", h.CreateWallet)
		wallets.GET("", h.GetAllWallets)
		wallets.GET("/total-balance", h.GetTotalBalance)
		wallets.GET("/:id", h.GetWalletByID)
		wallets.PUT("/:id", h.UpdateWallet)
		wallets.DELETE("/:id", h.DeleteWallet)
		wallets.POST("/:id/adjust", h.AdjustBalance)
		wallets.GET("/:id/adjustments", h.ListAdjustments)
		wallets.GET("/:id/history", h.History)
	}
}

func walletIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID dompet tidak valid")
		return uuid.Nil, false
	}
	return id, true
}

// POST /api/wallets
func (h *WalletHandler) CreateWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var req dto.CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.walletService.Create(userID, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusCreated, "dompet berhasil dibuat", result)
}

// GET /api/wallets?include_archived=true
func (h *WalletHandler) GetAllWallets(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var filter dto.WalletFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	results, err := h.walletService.GetAll(userID, filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "data dompet berhasil diambil", results)
}

// GET /api/wallets/total-balance
func (h *WalletHandler) GetTotalBalance(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	total, err := h.walletService.GetTotalBalance(userID)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "total saldo berhasil diambil", gin.H{"total_balance": total})
}

// GET /api/wallets/:id
func (h *WalletHandler) GetWalletByID(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := walletIDParam(c)
	if !ok {
		return
	}

	result, err := h.walletService.GetByID(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "detail dompet berhasil diambil", result)
}

// PUT /api/wallets/:id
func (h *WalletHandler) UpdateWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := walletIDParam(c)
	if !ok {
		return
	}

	var req dto.UpdateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.walletService.Update(userID, id, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "dompet berhasil diperbarui", result)
}

// DELETE /api/wallets/:id
func (h *WalletHandler) DeleteWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := walletIDParam(c)
	if !ok {
		return
	}

	archived, err := h.walletService.Delete(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	message := "dompet berhasil dihapus"
	if archived {
		message = "dompet punya riwayat transaksi, jadi diarsipkan agar laporan tetap utuh"
	}
	successResponse(c, http.StatusOK, message, dto.DeleteWalletResponse{Archived: archived})
}

// POST /api/wallets/:id/adjust
func (h *WalletHandler) AdjustBalance(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := walletIDParam(c)
	if !ok {
		return
	}

	var req dto.AdjustBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.walletService.Adjust(userID, id, req)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusCreated, "saldo berhasil disesuaikan", result)
}

// GET /api/wallets/:id/adjustments
func (h *WalletHandler) ListAdjustments(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := walletIDParam(c)
	if !ok {
		return
	}

	results, err := h.walletService.ListAdjustments(userID, id)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "riwayat penyesuaian saldo berhasil diambil", results)
}

// GET /api/wallets/:id/history?start_date=&end_date=&page=&limit=
func (h *WalletHandler) History(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}
	id, ok := walletIDParam(c)
	if !ok {
		return
	}

	var filter dto.WalletHistoryFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.walletService.History(userID, id, filter)
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	successResponse(c, http.StatusOK, "riwayat dompet berhasil diambil", result)
}
