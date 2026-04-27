// api/handler/wallet_handler.go
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
		wallets.POST("",              h.CreateWallet)
		wallets.GET("",               h.GetAllWallets)
		wallets.GET("/total-balance", h.GetTotalBalance)
		wallets.GET("/:id",           h.GetWalletByID)
		wallets.PUT("/:id",           h.UpdateWallet)
		wallets.DELETE("/:id",        h.DeleteWallet)
	}
}

// ─── POST /wallets ────────────────────────────────────────────────────────────

func (h *WalletHandler) CreateWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	var req dto.CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.walletService.Create(userID, req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusCreated, "dompet berhasil dibuat", result)
}

// ─── GET /wallets ─────────────────────────────────────────────────────────────

func (h *WalletHandler) GetAllWallets(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	results, err := h.walletService.GetAll(userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "data dompet berhasil diambil", results)
}

// ─── GET /wallets/total-balance ───────────────────────────────────────────────

func (h *WalletHandler) GetTotalBalance(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	total, err := h.walletService.GetTotalBalance(userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "total saldo berhasil diambil", gin.H{"total_balance": total})
}

// ─── GET /wallets/:id ─────────────────────────────────────────────────────────

func (h *WalletHandler) GetWalletByID(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID tidak valid")
		return
	}

	result, err := h.walletService.GetByID(userID, id)
	if err != nil {
		if err.Error() == "dompet tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "detail dompet berhasil diambil", result)
}

// ─── PUT /wallets/:id ─────────────────────────────────────────────────────────

func (h *WalletHandler) UpdateWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID tidak valid")
		return
	}

	var req dto.UpdateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.walletService.Update(userID, id, req)
	if err != nil {
		if err.Error() == "dompet tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "dompet berhasil diperbarui", result)
}

// ─── DELETE /wallets/:id ──────────────────────────────────────────────────────

func (h *WalletHandler) DeleteWallet(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID tidak valid")
		return
	}

	if err := h.walletService.Delete(userID, id); err != nil {
		if err.Error() == "dompet tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "dompet berhasil dihapus", nil)
}