package handler

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/service"

	"github.com/gin-gonic/gin"
)

type ExportHandler struct {
	transactionService service.TransactionService
	transferService    service.TransferService
	exportService      service.ExportService
}

func NewExportHandler(transactionService service.TransactionService, transferService service.TransferService, exportService service.ExportService) *ExportHandler {
	return &ExportHandler{transactionService: transactionService, transferService: transferService, exportService: exportService}
}

func (h *ExportHandler) RegisterRoutes(rg *gin.RouterGroup) {
	export := rg.Group("/export")
	{
		export.GET("/transactions.csv", h.Transactions)
		export.GET("/transfers.csv", h.Transfers)
		export.GET("/report.xlsx", h.Report)
	}
}

func money(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// writeCSV menulis BOM UTF-8 agar Excel membaca huruf non-ASCII dengan benar.
func writeCSV(c *gin.Context, filename string, header []string, rows [][]string) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Status(http.StatusOK)

	_, _ = c.Writer.Write([]byte("\xEF\xBB\xBF"))
	w := csv.NewWriter(c.Writer)
	_ = w.Write(header)
	_ = w.WriteAll(rows)
	w.Flush()
}

func exportFilename(prefix string, filter dto.ExportFilterRequest) string {
	name := prefix
	if filter.StartDate != nil {
		name += "_" + filter.StartDate.Format("20060102")
	}
	if filter.EndDate != nil {
		name += "_" + filter.EndDate.Format("20060102")
	}
	if filter.StartDate == nil && filter.EndDate == nil {
		name += "_" + time.Now().Format("20060102")
	}
	return name + ".csv"
}

// GET /api/export/transactions.csv?start_date=&end_date=
func (h *ExportHandler) Transactions(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var filter dto.ExportFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.transactionService.GetAllTransactions(userID, dto.TransactionFilterRequest{
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
	})
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	rows := make([][]string, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		t := items[i]
		rows = append(rows, []string{
			t.Date.Format("2006-01-02"), t.Type, t.CategoryName, t.WalletName, money(t.Amount), t.Note,
		})
	}

	writeCSV(c, exportFilename("transaksi", filter),
		[]string{"tanggal", "tipe", "kategori", "dompet", "nominal", "catatan"}, rows)
}

// GET /api/export/transfers.csv?start_date=&end_date=
func (h *ExportHandler) Transfers(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	var filter dto.ExportFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.transferService.GetAll(userID, dto.TransferFilterRequest{
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
	})
	if err != nil {
		serviceErrorResponse(c, err)
		return
	}

	rows := make([][]string, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		t := items[i]
		rows = append(rows, []string{
			t.Date.Format("2006-01-02"), t.FromWallet.Name, t.ToWallet.Name, money(t.Amount), money(t.Fee), t.Note,
		})
	}

	writeCSV(c, exportFilename("transfer", filter),
		[]string{"tanggal", "dari", "ke", "nominal", "biaya_admin", "catatan"}, rows)
}

// GET /api/export/report.xlsx?start=YYYY-MM-DD&end=YYYY-MM-DD
// Tanpa parameter: periode gajian yang sedang berjalan.
func (h *ExportHandler) Report(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		unauthorizedResponse(c)
		return
	}

	parse := func(key string) (*time.Time, bool) {
		v := c.Query(key)
		if v == "" {
			return nil, true
		}
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "format tanggal "+key+" harus YYYY-MM-DD")
			return nil, false
		}
		return &t, true
	}
	start, ok := parse("start")
	if !ok {
		return
	}
	end, ok := parse("end")
	if !ok {
		return
	}

	f, filename, err := h.exportService.Workbook(userID, start, end)
	if err != nil {
		if errors.Is(err, service.ErrExportRange) || errors.Is(err, service.ErrExportTooLarge) {
			errorResponse(c, http.StatusBadRequest, err.Error())
			return
		}
		serviceErrorResponse(c, err)
		return
	}
	defer f.Close()

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Status(http.StatusOK)
	if _, err := f.WriteTo(c.Writer); err != nil {
		log.Printf("[export xlsx] %v", err)
	}
}
