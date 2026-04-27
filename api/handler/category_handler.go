package handler

import (
	"dompetku/internal/dto"
	"dompetku/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

type CategoryHandler struct {
	categoryService service.CategoryService
}

type CategoryHandlerParams struct {
	fx.In
	CategoryService service.CategoryService
}

func NewCategoryHandler(p CategoryHandlerParams) *CategoryHandler {
	return &CategoryHandler{categoryService: p.CategoryService}
}

func (h *CategoryHandler) RegisterRoutes(rg *gin.RouterGroup) {
	categories := rg.Group("/categories")
	{
		categories.POST("", h.CreateCategory)
		categories.GET("", h.GetAllCategories)
		categories.GET("/:id", h.GetCategoryByID)
		categories.PUT("/:id", h.UpdateCategory)
		categories.DELETE("/:id", h.DeleteCategory)
	}
}

// CreateCategory godoc
// @Summary      Buat kategori baru
// @Tags         categories
// @Accept       json
// @Produce      json
// @Param        body body dto.CreateCategoryRequest true "Data kategori"
// @Success      201 {object} dto.CategoryResponse
// @Router       /categories [post]
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.categoryService.CreateCategory(userID, req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusCreated, "kategori berhasil dibuat", result)
}

// GetAllCategories godoc
// @Summary      Ambil semua kategori
// @Tags         categories
// @Produce      json
// @Param        type query string false "Filter tipe: income atau expense"
// @Success      200 {array} dto.CategoryResponse
// @Router       /categories [get]
func (h *CategoryHandler) GetAllCategories(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	// Query param ?type=income atau ?type=expense (opsional)
	var categoryType *string
	if t := c.Query("type"); t != "" {
		categoryType = &t
	}

	results, err := h.categoryService.GetAllCategories(userID, categoryType)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "data kategori berhasil diambil", results)
}

// GetCategoryByID godoc
// @Summary      Ambil detail kategori
// @Tags         categories
// @Produce      json
// @Param        id path string true "ID Kategori (UUID)"
// @Success      200 {object} dto.CategoryResponse
// @Router       /categories/{id} [get]
func (h *CategoryHandler) GetCategoryByID(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID kategori tidak valid")
		return
	}

	result, err := h.categoryService.GetCategoryByID(userID, id)
	if err != nil {
		if err.Error() == "kategori tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "detail kategori berhasil diambil", result)
}

// UpdateCategory godoc
// @Summary      Update kategori
// @Tags         categories
// @Accept       json
// @Produce      json
// @Param        id   path string true "ID Kategori (UUID)"
// @Param        body body dto.UpdateCategoryRequest true "Data yang ingin diubah"
// @Success      200 {object} dto.CategoryResponse
// @Router       /categories/{id} [put]
func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID kategori tidak valid")
		return
	}

	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.categoryService.UpdateCategory(userID, id, req)
	if err != nil {
		if err.Error() == "kategori tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "kategori berhasil diperbarui", result)
}

// DeleteCategory godoc
// @Summary      Hapus kategori
// @Tags         categories
// @Produce      json
// @Param        id path string true "ID Kategori (UUID)"
// @Success      200
// @Router       /categories/{id} [delete]
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		errorResponse(c, http.StatusUnauthorized, "tidak terotorisasi")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "format ID kategori tidak valid")
		return
	}

	if err := h.categoryService.DeleteCategory(userID, id); err != nil {
		if err.Error() == "kategori tidak ditemukan" {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, http.StatusOK, "kategori berhasil dihapus", nil)
}