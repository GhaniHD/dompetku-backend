package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

const (
	SheetTransactions = "transactions"
	SheetTransfers    = "transfers"
)

var ErrExportKind = errors.New("jenis ekspor tidak dikenal")

// kpi adalah satu kotak ringkasan di atas tabel. Formula memakai SUBTOTAL
// sehingga nilainya ikut berubah saat tabel difilter.
type kpi struct {
	label    string
	formula  string
	from, to int
	money    bool
}

func (sh *sheet) kpis(cards []kpi) {
	labelRow, valueRow := sh.row, sh.row+1
	for _, k := range cards {
		valueStyle := sh.st.kpiNum
		if k.money {
			valueStyle = sh.st.kpiMoney
		}
		for c := k.from; c <= k.to; c++ {
			sh.set(c, labelRow, "", sh.st.kpiLabel)
			sh.set(c, valueRow, "", valueStyle)
		}
		sh.set(k.from, labelRow, k.label, sh.st.kpiLabel)
		if sh.err == nil {
			sh.err = sh.f.SetCellFormula(sh.name, cell(k.from, valueRow), k.formula)
		}
		if sh.err == nil && k.to > k.from {
			sh.err = sh.f.MergeCell(sh.name, cell(k.from, labelRow), cell(k.to, labelRow))
			if sh.err == nil {
				sh.err = sh.f.MergeCell(sh.name, cell(k.from, valueRow), cell(k.to, valueRow))
			}
		}
	}
	if sh.err == nil {
		sh.err = sh.f.SetRowHeight(sh.name, labelRow, 20)
	}
	if sh.err == nil {
		sh.err = sh.f.SetRowHeight(sh.name, valueRow, 26)
	}
	sh.row = valueRow + 2
}

func (s *exportService) SingleSheet(userID uuid.UUID, kind string, start, end *time.Time) (*excelize.File, string, error) {
	if kind != SheetTransactions && kind != SheetTransfers {
		return nil, "", ErrExportKind
	}
	from, to, _, err := s.resolveRange(userID, start, end)
	if err != nil {
		return nil, "", err
	}
	user, err := s.userRepo.FindByID(context.Background(), userID)
	if err != nil || user == nil {
		return nil, "", ErrUserNotFound
	}
	wallets, err := s.walletRepo.FindAll(userID, true)
	if err != nil {
		return nil, "", err
	}
	names := make(map[uuid.UUID]string, len(wallets))
	for _, w := range wallets {
		names[w.ID] = w.Name
	}

	f := excelize.NewFile()
	st, err := newStyles(f)
	if err != nil {
		return nil, "", err
	}
	now := s.now().In(jakarta)
	startStr, endStr := from.Format("2006-01-02"), to.Format("2006-01-02")
	subtitle := fmt.Sprintf("%s · Periode %s sampai %s · Dibuat %s %02d:%02d WIB. Kotak ringkasan ikut berubah saat tabel difilter.",
		user.Name, idDate(from), idDate(to), idDate(now), now.Hour(), now.Minute())

	var sheetName, prefix string
	var render func(sh *sheet) error
	switch kind {
	case SheetTransactions:
		sheetName, prefix = "Transaksi", "myfulus-transaksi"
		render = func(sh *sheet) error {
			entries, err := s.repo.Entries(userID, startStr, endStr)
			if err != nil {
				return err
			}
			var txs []repository.ExportEntry
			for _, e := range entries {
				if e.Source == "transaction" {
					txs = append(txs, e)
				}
			}
			renderTransactionsSheet(sh, names, subtitle, txs)
			return nil
		}
	case SheetTransfers:
		sheetName, prefix = "Pindahan", "myfulus-pindahan"
		render = func(sh *sheet) error {
			endOfDay := to.Add(24*time.Hour - time.Second)
			transfers, err := s.transferRepo.FindAll(userID, dto.TransferFilterRequest{StartDate: &from, EndDate: &endOfDay})
			if err != nil {
				return err
			}
			sort.SliceStable(transfers, func(i, j int) bool {
				if !transfers[i].TransferDate.Equal(transfers[j].TransferDate) {
					return transfers[i].TransferDate.Before(transfers[j].TransferDate)
				}
				return transfers[i].CreatedAt.Before(transfers[j].CreatedAt)
			})
			renderTransfersSheet(sh, names, subtitle, transfers)
			return nil
		}
	}

	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, "", err
	}
	sh := &sheet{f: f, name: sheetName, st: st}
	if err := render(sh); err != nil {
		return nil, "", err
	}
	if sh.err != nil {
		return nil, "", fmt.Errorf("sheet %s: %w", sheetName, sh.err)
	}
	_ = f.SetDocProps(&excelize.DocProperties{Title: sheetName + " MyFulus", Creator: "MyFulus", Created: now.UTC().Format(time.RFC3339)})
	return f, fmt.Sprintf("%s_%s_%s.xlsx", prefix, startStr, endStr), nil
}

func renderTransactionsSheet(sh *sheet, names map[uuid.UUID]string, subtitle string, txs []repository.ExportEntry) {
	sh.heading("Transaksi MyFulus", subtitle)
	sh.row = 3
	sh.kpis([]kpi{
		{"Jumlah transaksi", "SUBTOTAL(103,Transaksi[Tanggal])", 1, 2, false},
		{"Total pemasukan", "SUBTOTAL(109,Transaksi[Pemasukan])", 3, 4, true},
		{"Total pengeluaran", "SUBTOTAL(109,Transaksi[Pengeluaran])", 5, 6, true},
		{"Selisih", "SUBTOTAL(109,Transaksi[Pemasukan])-SUBTOTAL(109,Transaksi[Pengeluaran])", 7, 7, true},
	})

	rows := make([][]any, 0, len(txs))
	for i, e := range txs {
		var in, out any = "", ""
		if e.Kind == "income" {
			in = round2(e.Amount)
		} else {
			out = round2(-e.Amount)
		}
		rows = append(rows, []any{
			i + 1, excelDate(e.Date), idDays[e.Date.Weekday()], entryKindLabel[e.Kind], e.CategoryName,
			names[e.WalletID], e.Note, in, out, sourceLabel(e), excelDateTime(e.CreatedAt),
		})
	}
	st := sh.st
	sh.table("Transaksi", []column{
		{"No", 6, st.num, ""}, {"Tanggal", 13, st.date, "count"}, {"Hari", 10, st.text, ""},
		{"Jenis", 13, st.text, ""}, {"Kategori", 20, st.text, ""}, {"Dompet", 20, st.text, ""},
		{"Catatan", 32, st.wrap, ""}, {"Pemasukan", 16, st.moneyPos, "sum"}, {"Pengeluaran", 16, st.moneyNeg, "sum"},
		{"Sumber", 20, st.text, ""}, {"Dicatat pada", 18, st.datetime, ""},
	}, rows, true)
	sh.layout(true)
}

func renderTransfersSheet(sh *sheet, names map[uuid.UUID]string, subtitle string, transfers []model.Transfer) {
	sh.heading("Pindahan MyFulus", subtitle+" Pindahan bukan pemasukan/pengeluaran; biaya admin dihitung sebagai pengeluaran.")
	sh.row = 3
	sh.kpis([]kpi{
		{"Jumlah pindahan", "SUBTOTAL(103,Pindahan[Tanggal])", 1, 2, false},
		{"Total dipindahkan", "SUBTOTAL(109,Pindahan[Nominal])", 3, 4, true},
		{"Total biaya admin", "SUBTOTAL(109,Pindahan[Biaya admin])", 5, 5, true},
		{"Total keluar dari asal", "SUBTOTAL(109,Pindahan[Total keluar dari asal])", 6, 7, true},
	})

	rows := make([][]any, 0, len(transfers))
	for i, t := range transfers {
		rows = append(rows, []any{
			i + 1, excelDate(t.TransferDate), idDays[t.TransferDate.Weekday()], nameOr(names, t.FromWalletID), nameOr(names, t.ToWalletID),
			round2(t.Amount), round2(t.Fee), round2(t.Amount + t.Fee), t.Note, excelDateTime(t.CreatedAt),
		})
	}
	st := sh.st
	sh.table("Pindahan", []column{
		{"No", 6, st.num, ""}, {"Tanggal", 13, st.date, "count"}, {"Hari", 10, st.text, ""},
		{"Dari dompet", 22, st.text, ""}, {"Ke dompet", 22, st.text, ""},
		{"Nominal", 16, st.money, "sum"}, {"Biaya admin", 14, st.money, "sum"}, {"Total keluar dari asal", 18, st.money, "sum"},
		{"Catatan", 32, st.wrap, ""}, {"Dicatat pada", 18, st.datetime, ""},
	}, rows, true)
	sh.layout(true)
}
