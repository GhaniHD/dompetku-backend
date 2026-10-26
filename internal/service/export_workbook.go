package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"dompetku/internal/repository"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

var (
	idMonths     = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	idMonthShort = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	idDays       = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

	walletTypeLabel = map[string]string{
		"cash": "Tunai", "bank": "Rekening bank", "ewallet": "Dompet digital",
		"credit_card": "Kartu kredit", "savings": "Tabungan",
	}
	entryKindLabel = map[string]string{
		"income": "Pemasukan", "expense": "Pengeluaran",
		"transfer_in": "Pindahan masuk", "transfer_out": "Pindahan keluar",
		"adjustment": "Penyesuaian saldo", "debt_payable": "Hutang", "debt_receivable": "Piutang",
	}
)

func idDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), idMonthShort[t.Month()-1], t.Year())
}

// styles menyimpan ID style excelize yang dipakai berulang.
type styles struct {
	title, subtitle, section, header, text, wrap, muted, aside, date, datetime          int
	money, moneyBold, moneyPos, moneyNeg, num, numBold, pct, pctBold, label, totalLabel int
	good, warn, bad, kpiLabel, kpiMoney, kpiNum                                         int
}

const (
	brand     = "6A3DE8"
	brandSoft = "F0EBFE"
	inkColor  = "1D1931"
	mutedText = "6B6680"
	lineColor = "E8E5F0"
	moneyFmt  = `"Rp"#,##0;[Red]-"Rp"#,##0;"Rp"0`
	dateFmt   = `dd mmm yyyy`
)

func newStyles(f *excelize.File) (*styles, error) {
	s := &styles{}
	font := func(bold bool, size float64, color string) *excelize.Font {
		return &excelize.Font{Bold: bold, Size: size, Color: color, Family: "Calibri"}
	}
	str := func(v string) *string { return &v }
	top := []excelize.Border{{Type: "top", Color: inkColor, Style: 1}}
	defs := []struct {
		id *int
		st excelize.Style
	}{
		{&s.title, excelize.Style{Font: font(true, 18, brand)}},
		{&s.subtitle, excelize.Style{Font: font(false, 11, mutedText)}},
		{&s.section, excelize.Style{Font: font(true, 13, inkColor), Border: []excelize.Border{{Type: "bottom", Color: brand, Style: 2}}}},
		{&s.header, excelize.Style{
			Font:      font(true, 11, "FFFFFF"),
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{brand}},
			Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		}},
		{&s.text, excelize.Style{Font: font(false, 11, inkColor), Alignment: &excelize.Alignment{Vertical: "top", Indent: 1}}},
		{&s.wrap, excelize.Style{Font: font(false, 11, inkColor), Alignment: &excelize.Alignment{Vertical: "top", WrapText: true, Indent: 1}}},
		{&s.muted, excelize.Style{Font: font(false, 10, mutedText), Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}}},
		{&s.aside, excelize.Style{Font: font(false, 10, mutedText), Alignment: &excelize.Alignment{Vertical: "top"}}},
		{&s.date, excelize.Style{Font: font(false, 11, inkColor), CustomNumFmt: str(dateFmt), Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "top", Indent: 1}}},
		{&s.datetime, excelize.Style{Font: font(false, 10, mutedText), CustomNumFmt: str("dd mmm yyyy hh:mm"), Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "top", Indent: 1}}},
		{&s.money, excelize.Style{Font: font(false, 11, inkColor), CustomNumFmt: str(moneyFmt), Alignment: &excelize.Alignment{Vertical: "top"}}},
		{&s.moneyBold, excelize.Style{Font: font(true, 11, inkColor), CustomNumFmt: str(moneyFmt), Border: top}},
		{&s.moneyPos, excelize.Style{Font: font(false, 11, "0F9D6B"), CustomNumFmt: str(moneyFmt), Alignment: &excelize.Alignment{Vertical: "top"}}},
		{&s.moneyNeg, excelize.Style{Font: font(false, 11, inkColor), CustomNumFmt: str(moneyFmt), Alignment: &excelize.Alignment{Vertical: "top"}}},
		{&s.num, excelize.Style{Font: font(false, 11, inkColor), CustomNumFmt: str("#,##0"), Alignment: &excelize.Alignment{Vertical: "top"}}},
		{&s.numBold, excelize.Style{Font: font(true, 11, inkColor), CustomNumFmt: str("#,##0"), Border: top}},
		{&s.pct, excelize.Style{Font: font(false, 11, inkColor), CustomNumFmt: str("0.0%"), Alignment: &excelize.Alignment{Vertical: "top"}}},
		{&s.pctBold, excelize.Style{Font: font(true, 11, inkColor), CustomNumFmt: str("0.0%"), Border: top}},
		{&s.label, excelize.Style{Font: font(true, 11, inkColor), Alignment: &excelize.Alignment{Vertical: "top"}}},
		{&s.totalLabel, excelize.Style{Font: font(true, 11, inkColor), Border: top, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{brandSoft}}}},
		{&s.good, excelize.Style{Font: font(true, 11, "0F9D6B")}},
		{&s.warn, excelize.Style{Font: font(true, 11, "C48008")}},
		{&s.bad, excelize.Style{Font: font(true, 11, "E0484E")}},
		{&s.kpiLabel, excelize.Style{Font: font(false, 10, mutedText), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{brandSoft}}, Alignment: &excelize.Alignment{Vertical: "bottom"}}},
		{&s.kpiMoney, excelize.Style{Font: font(true, 14, inkColor), CustomNumFmt: str(moneyFmt), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{brandSoft}}, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"}}},
		{&s.kpiNum, excelize.Style{Font: font(true, 14, inkColor), CustomNumFmt: str("#,##0"), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{brandSoft}}, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"}}},
	}
	for _, d := range defs {
		id, err := f.NewStyle(&d.st)
		if err != nil {
			return nil, err
		}
		*d.id = id
	}
	return s, nil
}

// column mendeskripsikan satu kolom tabel: judul, lebar, dan style isi.
type column struct {
	title string
	width float64
	style int
	total string // "sum", "count", atau "" (tanpa total)
}

type sheet struct {
	f    *excelize.File
	name string
	st   *styles
	row  int
	err  error
}

func cell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

func (sh *sheet) set(col, row int, value any, style int) {
	if sh.err != nil {
		return
	}
	c := cell(col, row)
	if err := sh.f.SetCellValue(sh.name, c, value); err != nil {
		sh.err = err
		return
	}
	if style != 0 {
		sh.err = sh.f.SetCellStyle(sh.name, c, c, style)
	}
}

func (sh *sheet) heading(title, subtitle string) {
	sh.set(1, 1, title, sh.st.title)
	if sh.err == nil {
		sh.err = sh.f.SetRowHeight(sh.name, 1, 28)
	}
	sh.set(1, 2, subtitle, sh.st.subtitle)
	sh.row = 4
}

func (sh *sheet) section(title string, span int) {
	sh.set(1, sh.row, title, sh.st.section)
	for c := 2; c <= span; c++ {
		sh.set(c, sh.row, "", sh.st.section)
	}
	sh.row += 2
}

func (sh *sheet) note(text string, span int) {
	sh.set(1, sh.row, text, sh.st.muted)
	if sh.err == nil && span > 1 {
		sh.err = sh.f.MergeCell(sh.name, cell(1, sh.row), cell(span, sh.row))
	}
	if sh.err == nil {
		lines := math.Ceil(float64(len(text)) / 120)
		sh.err = sh.f.SetRowHeight(sh.name, sh.row, math.Max(16, 15*lines))
	}
	sh.row++
}

// table menulis header + baris sebagai Excel Table (bisa difilter & diurutkan),
// lalu baris total memakai SUBTOTAL agar ikut menyesuaikan saat difilter.
// Mengembalikan nomor baris pertama dan terakhir data.
func (sh *sheet) table(name string, cols []column, rows [][]any, freeze bool) (int, int) {
	if sh.err != nil {
		return 0, 0
	}
	headerRow := sh.row
	for i, c := range cols {
		sh.set(i+1, headerRow, c.title, sh.st.header)
		col, _ := excelize.ColumnNumberToName(i + 1)
		if sh.err == nil && c.width > 0 {
			sh.err = sh.f.SetColWidth(sh.name, col, col, math.Max(c.width, currentWidth(sh, col)))
		}
	}
	if sh.err == nil {
		sh.err = sh.f.SetRowHeight(sh.name, headerRow, 30)
	}

	if len(rows) == 0 {
		empty := make([]any, len(cols))
		empty[0] = "Tidak ada data pada periode ini"
		rows = [][]any{empty}
	}
	first := headerRow + 1
	for r, values := range rows {
		for c, v := range values {
			if v == nil {
				v = ""
			}
			sh.set(c+1, first+r, v, cols[c].style)
		}
	}
	last := first + len(rows) - 1

	if sh.err == nil {
		yes := true
		sh.err = sh.f.AddTable(sh.name, &excelize.Table{
			Range:          cell(1, headerRow) + ":" + cell(len(cols), last),
			Name:           name,
			StyleName:      "TableStyleLight15",
			ShowRowStripes: &yes,
		})
	}

	totalRow := last + 1
	hasTotal := false
	for i, c := range cols {
		if c.total == "" {
			continue
		}
		hasTotal = true
		ref := fmt.Sprintf("%s[%s]", name, escapeTableColumn(c.title))
		fn := 109
		style := sh.st.moneyBold
		if c.total == "count" {
			fn, style = 103, sh.st.numBold
		}
		if c.style == sh.st.num {
			style = sh.st.numBold
		}
		if sh.err == nil {
			sh.err = sh.f.SetCellFormula(sh.name, cell(i+1, totalRow), fmt.Sprintf("SUBTOTAL(%d,%s)", fn, ref))
		}
		if sh.err == nil {
			sh.err = sh.f.SetCellStyle(sh.name, cell(i+1, totalRow), cell(i+1, totalRow), style)
		}
	}
	if hasTotal {
		sh.set(1, totalRow, "Total", sh.st.totalLabel)
		for i, c := range cols {
			if i > 0 && c.total == "" {
				sh.set(i+1, totalRow, "", sh.st.totalLabel)
			}
		}
		sh.row = totalRow + 2
	} else {
		sh.row = last + 2
	}

	if freeze && sh.err == nil {
		sh.err = sh.f.SetPanes(sh.name, &excelize.Panes{
			Freeze: true, YSplit: headerRow, TopLeftCell: cell(1, headerRow+1), ActivePane: "bottomLeft",
		})
	}
	return first, last
}

func currentWidth(sh *sheet, col string) float64 {
	w, err := sh.f.GetColWidth(sh.name, col)
	if err != nil {
		return 0
	}
	return w
}

// Karakter khusus pada nama kolom Excel Table harus di-escape dengan apostrof.
func escapeTableColumn(s string) string {
	r := strings.NewReplacer("'", "''", "[", "'[", "]", "']", "#", "'#")
	return r.Replace(s)
}

func (sh *sheet) keyValues(pairs [][3]any) {
	for _, p := range pairs {
		sh.set(1, sh.row, p[0], sh.st.label)
		style := sh.st.money
		switch v := p[1].(type) {
		case string:
			style = sh.st.text
		case int, int64:
			style = sh.st.num
		case percent:
			p[1], style = float64(v), sh.st.pct
		case time.Time:
			style = sh.st.date
		}
		sh.set(2, sh.row, p[1], style)
		if p[2] != nil && p[2] != "" {
			sh.set(3, sh.row, p[2], sh.st.aside)
		}
		sh.row++
	}
	sh.row++
}

type percent float64

func (sh *sheet) layout(landscape bool) {
	if sh.err != nil {
		return
	}
	orientation := "portrait"
	if landscape {
		orientation = "landscape"
	}
	one, zero := 1, 0
	sh.err = sh.f.SetPageLayout(sh.name, &excelize.PageLayoutOptions{Orientation: &orientation, FitToWidth: &one, FitToHeight: &zero})
	if sh.err == nil {
		fit := true
		sh.err = sh.f.SetSheetProps(sh.name, &excelize.SheetPropsOptions{FitToPage: &fit, TabColorRGB: strPtr(brand)})
	}
	if sh.err == nil {
		sh.err = sh.f.SetHeaderFooter(sh.name, &excelize.HeaderFooterOptions{OddFooter: "&LDompetKu&CHalaman &P dari &N&R&A"})
	}
}

func strPtr(s string) *string { return &s }

func excelDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func excelDateTime(t time.Time) time.Time {
	l := t.In(jakarta)
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), 0, time.UTC)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ─── Agregasi ────────────────────────────────────────────────────────────────

type walletFlow struct {
	opening, income, expense, transferIn, transferOut, adjustment, debt float64
}

func (w walletFlow) closing() float64 {
	return w.opening + w.income - w.expense + w.transferIn - w.transferOut + w.adjustment + w.debt
}

type categoryStat struct {
	kind, name    string
	count         int
	total, maxAmt float64
	minAmt        float64
}

func aggregate(d *reportData) (map[uuid.UUID]*walletFlow, []repository.ExportEntry, map[string]*categoryStat) {
	flows := make(map[uuid.UUID]*walletFlow, len(d.Wallets))
	for _, w := range d.Wallets {
		flows[w.ID] = &walletFlow{opening: d.OpeningByID[w.ID]}
	}
	var txs []repository.ExportEntry
	cats := map[string]*categoryStat{}
	for _, e := range d.Entries {
		fl, ok := flows[e.WalletID]
		if !ok {
			fl = &walletFlow{opening: d.OpeningByID[e.WalletID]}
			flows[e.WalletID] = fl
		}
		switch e.Kind {
		case "income":
			fl.income += e.Amount
		case "expense":
			fl.expense += -e.Amount
		case "transfer_in":
			fl.transferIn += e.Amount
		case "transfer_out":
			fl.transferOut += -e.Amount
		case "adjustment":
			fl.adjustment += e.Amount
		default:
			fl.debt += e.Amount
		}
		if e.Source != "transaction" {
			continue
		}
		txs = append(txs, e)
		amount := math.Abs(e.Amount)
		key := e.Kind + "|" + e.CategoryName
		c, ok := cats[key]
		if !ok {
			c = &categoryStat{kind: e.Kind, name: e.CategoryName, minAmt: amount}
			cats[key] = c
		}
		c.count++
		c.total += amount
		c.maxAmt = math.Max(c.maxAmt, amount)
		c.minAmt = math.Min(c.minAmt, amount)
	}
	return flows, txs, cats
}

func sourceLabel(e repository.ExportEntry) string {
	switch {
	case e.TransferFee:
		return "Biaya admin transfer"
	case e.Recurring:
		return "Transaksi rutin"
	default:
		return "Dicatat manual"
	}
}

// ─── Render ──────────────────────────────────────────────────────────────────

func renderWorkbook(d *reportData) (*excelize.File, error) {
	f := excelize.NewFile()
	st, err := newStyles(f)
	if err != nil {
		return nil, err
	}

	period := fmt.Sprintf("Periode %s sampai %s", idDate(d.Start), idDate(d.End))
	flows, txs, cats := aggregate(d)

	names := []string{"Ringkasan", "Transaksi", "Mutasi Dompet", "Pindahan", "Per Kategori", "Harian", "Bulanan", "Anggaran", "Hutang & Piutang", "Target Tabungan", "Transaksi Rutin"}
	if err := f.SetSheetName("Sheet1", names[0]); err != nil {
		return nil, err
	}
	for _, n := range names[1:] {
		if _, err := f.NewSheet(n); err != nil {
			return nil, err
		}
	}

	renders := []func(*sheet){
		func(sh *sheet) { renderSummary(sh, d, period, flows, txs, cats) },
		func(sh *sheet) { renderTransactions(sh, d, period, txs) },
		func(sh *sheet) { renderMutations(sh, d, period) },
		func(sh *sheet) { renderTransfers(sh, d, period) },
		func(sh *sheet) { renderCategories(sh, period, cats) },
		func(sh *sheet) { renderDaily(sh, d, period, txs) },
		func(sh *sheet) { renderMonthly(sh, d, period, txs) },
		func(sh *sheet) { renderBudgets(sh, d) },
		func(sh *sheet) { renderDebts(sh, d, period) },
		func(sh *sheet) { renderGoals(sh, d) },
		func(sh *sheet) { renderRecurring(sh, d) },
	}
	for i, render := range renders {
		sh := &sheet{f: f, name: names[i], st: st}
		render(sh)
		if sh.err != nil {
			return nil, fmt.Errorf("sheet %s: %w", names[i], sh.err)
		}
	}

	f.SetActiveSheet(0)
	_ = f.SetDocProps(&excelize.DocProperties{
		Title:   "Laporan Keuangan DompetKu",
		Subject: period,
		Creator: "DompetKu",
		Created: d.GeneratedAt.UTC().Format(time.RFC3339),
	})
	return f, nil
}

func renderSummary(sh *sheet, d *reportData, period string, flows map[uuid.UUID]*walletFlow, txs []repository.ExportEntry, cats map[string]*categoryStat) {
	sh.heading("Laporan Keuangan DompetKu", fmt.Sprintf("%s (%s). %s. Dibuat %s %02d:%02d WIB.",
		d.User.Name, d.User.Email, period, idDate(d.GeneratedAt), d.GeneratedAt.Hour(), d.GeneratedAt.Minute()))

	var income, expense, fees float64
	var incomeN, expenseN int
	var biggestIn, biggestOut *repository.ExportEntry
	daily := map[string]float64{}
	for i := range txs {
		e := &txs[i]
		if e.Kind == "income" {
			income += e.Amount
			incomeN++
			if biggestIn == nil || e.Amount > biggestIn.Amount {
				biggestIn = e
			}
			continue
		}
		expense += -e.Amount
		expenseN++
		daily[e.Date.Format("2006-01-02")] += -e.Amount
		if e.TransferFee {
			fees += -e.Amount
		}
		if biggestOut == nil || -e.Amount > -biggestOut.Amount {
			biggestOut = e
		}
	}
	var moved float64
	for _, t := range d.Transfers {
		moved += t.Amount
	}
	days := int(d.End.Sub(d.Start).Hours()/24) + 1
	var savings percent
	if income > 0 {
		savings = percent((income - expense) / income)
	}
	describe := func(e *repository.ExportEntry) string {
		if e == nil {
			return ""
		}
		note := e.Note
		if note == "" {
			note = e.CategoryName
		}
		return fmt.Sprintf("%s, %s, %s", note, d.WalletName[e.WalletID], idDate(e.Date))
	}
	amount := func(e *repository.ExportEntry) float64 {
		if e == nil {
			return 0
		}
		return math.Abs(e.Amount)
	}

	var topCat *categoryStat
	for _, c := range cats {
		if c.kind == "expense" && (topCat == nil || c.total > topCat.total) {
			topCat = c
		}
	}
	topCatName, topCatTotal := "", 0.0
	if topCat != nil {
		topCatName, topCatTotal = topCat.name, topCat.total
	}
	peakDay, peakAmt := "", 0.0
	for day, v := range daily {
		if v > peakAmt || (v == peakAmt && day < peakDay) {
			peakDay, peakAmt = day, v
		}
	}
	peakLabel := ""
	if peakDay != "" {
		t, _ := time.Parse("2006-01-02", peakDay)
		peakLabel = fmt.Sprintf("%s, %s", idDays[t.Weekday()], idDate(t))
	}

	sh.section("Arus kas", 4)
	sh.keyValues([][3]any{
		{"Pemasukan", round2(income), fmt.Sprintf("%d transaksi", incomeN)},
		{"Pengeluaran", round2(expense), fmt.Sprintf("%d transaksi, termasuk biaya admin transfer", expenseN)},
		{"Selisih (pemasukan − pengeluaran)", round2(income - expense), ""},
		{"Rasio tabungan", savings, "Bagian pemasukan yang tidak dibelanjakan"},
		{"Rata-rata pengeluaran per hari", round2(expense / float64(days)), fmt.Sprintf("Dari %d hari dalam periode", days)},
		{"Pengeluaran terbesar", amount(biggestOut), describe(biggestOut)},
		{"Pemasukan terbesar", amount(biggestIn), describe(biggestIn)},
		{"Kategori pengeluaran terbesar", round2(topCatTotal), topCatName},
		{"Hari paling boros", round2(peakAmt), peakLabel},
		{"Total biaya admin transfer", round2(fees), ""},
		{"Total uang dipindahkan antar dompet", round2(moved), fmt.Sprintf("%d kali pindahan, tidak dihitung sebagai pemasukan/pengeluaran", len(d.Transfers))},
	})

	sh.section("Posisi dompet", 12)
	sh.note("Saldo akhir = saldo awal + pemasukan − pengeluaran + pindahan masuk − pindahan keluar + penyesuaian + hutang/piutang. Setiap angka bisa ditelusuri di sheet Mutasi Dompet.", 12)
	sh.row++
	var rows [][]any
	var assets, liabilities float64
	for _, w := range d.Wallets {
		fl := flows[w.ID]
		status := "Aktif"
		if w.IsArchived {
			status = "Diarsipkan"
		}
		closing := round2(fl.closing())
		if closing >= 0 {
			assets += closing
		} else {
			liabilities += closing
		}
		rows = append(rows, []any{
			w.Name, walletTypeLabel[w.Type], status, round2(fl.opening), round2(fl.income), round2(fl.expense),
			round2(fl.transferIn), round2(fl.transferOut), round2(fl.adjustment), round2(fl.debt), closing, round2(closing - fl.opening),
		})
	}
	m, b := sh.st.money, sh.st.text
	sh.table("PosisiDompet", []column{
		{"Dompet", 24, b, ""}, {"Jenis", 16, b, ""}, {"Status", 12, b, ""},
		{"Saldo awal", 16, m, "sum"}, {"Pemasukan", 16, m, "sum"}, {"Pengeluaran", 16, m, "sum"},
		{"Pindahan masuk", 16, m, "sum"}, {"Pindahan keluar", 16, m, "sum"}, {"Penyesuaian", 15, m, "sum"},
		{"Hutang & piutang", 16, m, "sum"}, {"Saldo akhir", 17, m, "sum"}, {"Perubahan", 16, m, "sum"},
	}, rows, false)

	var payable, receivable float64
	if d.Debts != nil {
		payable, receivable = d.Debts.TotalPayable, d.Debts.TotalReceivable
	}
	sh.section("Kekayaan bersih", 4)
	sh.keyValues([][3]any{
		{"Saldo dompet positif (aset)", round2(assets), "Per akhir periode"},
		{"Saldo minus kartu kredit", round2(liabilities), "Per akhir periode"},
		{"Sisa hutang yang belum dibayar", round2(-payable), "Per hari laporan dibuat"},
		{"Piutang yang belum diterima", round2(receivable), "Per hari laporan dibuat"},
		{"Kekayaan bersih", round2(assets + liabilities - payable + receivable), "Aset + kartu kredit − hutang + piutang"},
	})

	sh.section("Isi laporan", 4)
	guide := [][2]string{
		{"Transaksi", "Semua pemasukan dan pengeluaran, termasuk transaksi rutin otomatis dan biaya admin transfer."},
		{"Mutasi Dompet", "Setiap perubahan saldo per dompet beserta saldo setelahnya, seperti rekening koran."},
		{"Pindahan", "Pindahan uang antar dompet beserta biaya adminnya."},
		{"Per Kategori", "Jumlah, total, rata-rata, terbesar, dan terkecil per kategori."},
		{"Harian & Bulanan", "Pemasukan dan pengeluaran per hari dan per bulan, lengkap dengan grafik."},
		{"Anggaran", "Batas, pemakaian, dan sisa anggaran setiap periode anggaran yang beririsan dengan laporan."},
		{"Hutang & Piutang", "Posisi seluruh hutang/piutang saat ini dan pembayaran dalam periode."},
		{"Target Tabungan & Transaksi Rutin", "Kondisi target tabungan dan jadwal transaksi rutin saat laporan dibuat."},
	}
	for _, g := range guide {
		sh.set(1, sh.row, g[0], sh.st.label)
		sh.set(2, sh.row, g[1], sh.st.text)
		sh.row++
	}
	if sh.err == nil {
		sh.err = sh.f.SetColWidth(sh.name, "A", "A", 34)
	}
	if sh.err == nil {
		sh.err = sh.f.SetColWidth(sh.name, "B", "B", 18)
	}
	sh.layout(true)
}

func renderTransactions(sh *sheet, d *reportData, period string, txs []repository.ExportEntry) {
	sh.heading("Transaksi", period+". Biaya admin transfer dan transaksi rutin otomatis ikut tercatat.")
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
			d.WalletName[e.WalletID], e.Note, in, out, sourceLabel(e), excelDateTime(e.CreatedAt),
		})
	}
	st := sh.st
	sh.table("Transaksi", []column{
		{"No", 6, st.num, ""}, {"Tanggal", 13, st.date, ""}, {"Hari", 10, st.text, ""},
		{"Jenis", 13, st.text, ""}, {"Kategori", 20, st.text, ""}, {"Dompet", 20, st.text, ""},
		{"Catatan", 32, st.wrap, ""}, {"Pemasukan", 16, st.moneyPos, "sum"}, {"Pengeluaran", 16, st.moneyNeg, "sum"},
		{"Sumber", 20, st.text, ""}, {"Dicatat pada", 18, st.datetime, ""},
	}, rows, true)
	sh.layout(true)
}

func renderMutations(sh *sheet, d *reportData, period string) {
	sh.heading("Mutasi Dompet", period+". Diurutkan per dompet lalu tanggal; baris pertama tiap dompet adalah saldo awal periode.")
	byWallet := map[uuid.UUID][]repository.ExportEntry{}
	for _, e := range d.Entries {
		byWallet[e.WalletID] = append(byWallet[e.WalletID], e)
	}
	var rows [][]any
	for _, w := range d.Wallets {
		balance := d.OpeningByID[w.ID]
		entries := byWallet[w.ID]
		rows = append(rows, []any{w.Name, excelDate(d.Start), "Saldo awal periode", "", "", "", "", round2(balance)})
		for _, e := range entries {
			balance += e.Amount
			detail := e.CategoryName
			switch e.Kind {
			case "transfer_in":
				detail = "Dari " + e.Counterpart
			case "transfer_out":
				detail = "Ke " + e.Counterpart
			case "debt_payable", "debt_receivable":
				detail = e.Counterpart
			}
			var in, out any = "", ""
			if e.Amount >= 0 {
				in = round2(e.Amount)
			} else {
				out = round2(-e.Amount)
			}
			kind := entryKindLabel[e.Kind]
			if e.TransferFee {
				kind = "Biaya admin transfer"
			}
			rows = append(rows, []any{w.Name, excelDate(e.Date), kind, detail, e.Note, in, out, round2(balance)})
		}
	}
	st := sh.st
	sh.table("MutasiDompet", []column{
		{"Dompet", 22, st.text, ""}, {"Tanggal", 13, st.date, ""}, {"Jenis mutasi", 20, st.text, ""},
		{"Kategori / pihak", 22, st.text, ""}, {"Catatan", 32, st.wrap, ""},
		{"Masuk", 16, st.moneyPos, "sum"}, {"Keluar", 16, st.moneyNeg, "sum"}, {"Saldo setelahnya", 18, st.money, ""},
	}, rows, true)
	sh.layout(true)
}

func renderTransfers(sh *sheet, d *reportData, period string) {
	sh.heading("Pindahan antar dompet", period+". Pindahan tidak dihitung sebagai pemasukan atau pengeluaran; biaya admin dihitung sebagai pengeluaran.")
	rows := make([][]any, 0, len(d.Transfers))
	for i, t := range d.Transfers {
		rows = append(rows, []any{
			i + 1, excelDate(t.TransferDate), nameOr(d.WalletName, t.FromWalletID), nameOr(d.WalletName, t.ToWalletID),
			round2(t.Amount), round2(t.Fee), round2(t.Amount + t.Fee), t.Note, excelDateTime(t.CreatedAt),
		})
	}
	st := sh.st
	sh.table("Pindahan", []column{
		{"No", 6, st.num, ""}, {"Tanggal", 13, st.date, ""}, {"Dari dompet", 22, st.text, ""}, {"Ke dompet", 22, st.text, ""},
		{"Nominal", 16, st.money, "sum"}, {"Biaya admin", 14, st.money, "sum"}, {"Total keluar dari asal", 18, st.money, "sum"},
		{"Catatan", 32, st.wrap, ""}, {"Dicatat pada", 18, st.datetime, ""},
	}, rows, true)
	sh.layout(true)
}

func nameOr(names map[uuid.UUID]string, id uuid.UUID) string {
	if n, ok := names[id]; ok {
		return n
	}
	return "(dompet dihapus)"
}

func renderCategories(sh *sheet, period string, cats map[string]*categoryStat) {
	sh.heading("Per Kategori", period+".")
	list := make([]*categoryStat, 0, len(cats))
	totals := map[string]float64{}
	for _, c := range cats {
		list = append(list, c)
		totals[c.kind] += c.total
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].kind != list[j].kind {
			return list[i].kind == "expense"
		}
		return list[i].total > list[j].total
	})
	rows := make([][]any, 0, len(list))
	expenseRows := 0
	for _, c := range list {
		share := 0.0
		if totals[c.kind] > 0 {
			share = c.total / totals[c.kind]
		}
		if c.kind == "expense" {
			expenseRows++
		}
		rows = append(rows, []any{
			entryKindLabel[c.kind], c.name, c.count, round2(c.total), share,
			round2(c.total / float64(c.count)), round2(c.maxAmt), round2(c.minAmt),
		})
	}
	st := sh.st
	first, _ := sh.table("PerKategori", []column{
		{"Jenis", 13, st.text, ""}, {"Kategori", 24, st.text, ""}, {"Jumlah transaksi", 12, st.num, "sum"},
		{"Total", 17, st.money, ""}, {"Porsi dari jenisnya", 12, st.pct, ""}, {"Rata-rata per transaksi", 16, st.money, ""},
		{"Terbesar", 15, st.money, ""}, {"Terkecil", 15, st.money, ""},
	}, rows, true)

	if expenseRows > 0 && sh.err == nil {
		last := first + expenseRows - 1
		sh.err = sh.f.AddChart(sh.name, "J4", &excelize.Chart{
			Type: excelize.Bar,
			Series: []excelize.ChartSeries{{
				Name:       "Pengeluaran",
				Categories: fmt.Sprintf("'%s'!$B$%d:$B$%d", sh.name, first, last),
				Values:     fmt.Sprintf("'%s'!$D$%d:$D$%d", sh.name, first, last),
				Fill:       excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{brand}},
			}},
			Title:     []excelize.RichTextRun{{Text: "Pengeluaran per kategori"}},
			Legend:    excelize.ChartLegend{Position: "none"},
			YAxis:     excelize.ChartAxis{ReverseOrder: true, MajorGridLines: false},
			XAxis:     excelize.ChartAxis{MajorGridLines: true, NumFmt: excelize.ChartNumFmt{CustomNumFmt: `"Rp"#,##0`}},
			PlotArea:  excelize.ChartPlotArea{ShowVal: false},
			Dimension: excelize.ChartDimension{Width: 560, Height: uint(math.Max(260, float64(60+28*expenseRows)))},
		})
	}
	sh.layout(true)
}

func renderDaily(sh *sheet, d *reportData, period string, txs []repository.ExportEntry) {
	sh.heading("Harian", period+". Semua hari ditampilkan, termasuk hari tanpa transaksi.")
	type day struct {
		in, out float64
		n       int
	}
	byDay := map[string]*day{}
	for _, e := range txs {
		k := e.Date.Format("2006-01-02")
		dd, ok := byDay[k]
		if !ok {
			dd = &day{}
			byDay[k] = dd
		}
		dd.n++
		if e.Kind == "income" {
			dd.in += e.Amount
		} else {
			dd.out += -e.Amount
		}
	}
	var rows [][]any
	cumulative := 0.0
	for t := d.Start; !t.After(d.End); t = t.AddDate(0, 0, 1) {
		dd := byDay[t.Format("2006-01-02")]
		if dd == nil {
			dd = &day{}
		}
		cumulative += dd.in - dd.out
		rows = append(rows, []any{excelDate(t), idDays[t.Weekday()], round2(dd.in), round2(dd.out), round2(dd.in - dd.out), round2(cumulative), dd.n})
	}
	st := sh.st
	first, last := sh.table("Harian", []column{
		{"Tanggal", 13, st.date, ""}, {"Hari", 10, st.text, ""}, {"Pemasukan", 16, st.moneyPos, "sum"},
		{"Pengeluaran", 16, st.moneyNeg, "sum"}, {"Selisih", 16, st.money, "sum"}, {"Selisih kumulatif", 18, st.money, ""},
		{"Jumlah transaksi", 12, st.num, "sum"},
	}, rows, true)
	if sh.err == nil && len(rows) > 1 {
		sh.err = sh.f.AddChart(sh.name, "I4", &excelize.Chart{
			Type: excelize.Col,
			Series: []excelize.ChartSeries{{
				Name:       "Pengeluaran",
				Categories: fmt.Sprintf("'%s'!$A$%d:$A$%d", sh.name, first, last),
				Values:     fmt.Sprintf("'%s'!$D$%d:$D$%d", sh.name, first, last),
				Fill:       excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{brand}},
			}},
			Title:     []excelize.RichTextRun{{Text: "Pengeluaran harian"}},
			Legend:    excelize.ChartLegend{Position: "none"},
			XAxis:     excelize.ChartAxis{NumFmt: excelize.ChartNumFmt{CustomNumFmt: "d mmm"}},
			YAxis:     excelize.ChartAxis{MajorGridLines: true, NumFmt: excelize.ChartNumFmt{CustomNumFmt: `"Rp"#,##0`}},
			Dimension: excelize.ChartDimension{Width: 720, Height: 300},
		})
	}
	sh.layout(true)
}

func renderMonthly(sh *sheet, d *reportData, period string, txs []repository.ExportEntry) {
	sh.heading("Bulanan", period+". Dikelompokkan per bulan kalender.")
	type month struct{ in, out float64 }
	byMonth := map[string]*month{}
	for _, e := range txs {
		k := e.Date.Format("2006-01")
		m, ok := byMonth[k]
		if !ok {
			m = &month{}
			byMonth[k] = m
		}
		if e.Kind == "income" {
			m.in += e.Amount
		} else {
			m.out += -e.Amount
		}
	}
	var rows [][]any
	for t := time.Date(d.Start.Year(), d.Start.Month(), 1, 0, 0, 0, 0, time.UTC); !t.After(d.End); t = t.AddDate(0, 1, 0) {
		m := byMonth[t.Format("2006-01")]
		if m == nil {
			m = &month{}
		}
		rate := 0.0
		if m.in > 0 {
			rate = (m.in - m.out) / m.in
		}
		rows = append(rows, []any{fmt.Sprintf("%s %d", idMonths[t.Month()-1], t.Year()), round2(m.in), round2(m.out), round2(m.in - m.out), rate})
	}
	st := sh.st
	sh.table("Bulanan", []column{
		{"Bulan", 18, st.text, ""}, {"Pemasukan", 17, st.moneyPos, "sum"}, {"Pengeluaran", 17, st.moneyNeg, "sum"},
		{"Selisih", 17, st.money, "sum"}, {"Rasio tabungan", 14, st.pct, ""},
	}, rows, true)
	sh.layout(false)
}

func renderBudgets(sh *sheet, d *reportData) {
	basis := fmt.Sprintf("Periode anggaran mengikuti tanggal gajian %d.", d.BudgetCycleD)
	if d.BudgetCycleD == 1 {
		basis = "Periode anggaran mengikuti bulan kalender."
	}
	sh.heading("Anggaran", basis+" Pemakaian dihitung untuk seluruh periode anggaran.")
	statusLabel := map[string]string{"on_track": "Aman", "warning": "Hampir habis", "over_budget": "Terlampaui"}
	var rows [][]any
	var statuses []string
	for _, summary := range d.Budgets {
		start, _ := time.Parse("2006-01-02", summary.PeriodStart)
		end, _ := time.Parse("2006-01-02", summary.PeriodEnd)
		label := fmt.Sprintf("%s %d", idMonths[summary.Month-1], summary.Year)
		for _, b := range summary.Budgets {
			rows = append(rows, []any{
				label, excelDate(start), excelDate(end), b.Category.Name, round2(b.Amount), round2(b.Spent),
				round2(b.Remain), b.UsedPct / 100, statusLabel[b.Status], b.Notes,
			})
			statuses = append(statuses, b.Status)
		}
	}
	st := sh.st
	first, _ := sh.table("Anggaran", []column{
		{"Periode", 16, st.text, ""}, {"Mulai", 13, st.date, ""}, {"Sampai", 13, st.date, ""}, {"Kategori", 22, st.text, ""},
		{"Anggaran", 16, st.money, "sum"}, {"Terpakai", 16, st.money, "sum"}, {"Sisa", 16, st.money, "sum"},
		{"Pemakaian", 12, st.pct, ""}, {"Status", 14, st.text, ""}, {"Catatan", 28, st.wrap, ""},
	}, rows, true)
	for i, s := range statuses {
		style := st.good
		switch s {
		case "warning":
			style = st.warn
		case "over_budget":
			style = st.bad
		}
		if sh.err == nil {
			sh.err = sh.f.SetCellStyle(sh.name, cell(9, first+i), cell(9, first+i), style)
		}
	}
	if len(rows) > 0 && sh.err == nil {
		sh.err = sh.f.SetConditionalFormat(sh.name, fmt.Sprintf("H%d:H%d", first, first+len(rows)-1), []excelize.ConditionalFormatOptions{{
			Type: "data_bar", Criteria: "=", MinType: "num", MinValue: "0", MaxType: "num", MaxValue: "1", BarColor: "#B9A6F5", BarSolid: true,
		}})
	}
	sh.layout(true)
}

func renderDebts(sh *sheet, d *reportData, period string) {
	sh.heading("Hutang & Piutang", "Posisi seluruh catatan per hari laporan dibuat, lalu pembayaran yang terjadi dalam periode.")
	kindLabel := map[string]string{"payable": "Hutang saya", "receivable": "Piutang"}
	var rows [][]any
	var overdue []bool
	if d.Debts != nil {
		for _, x := range d.Debts.Debts {
			status := "Belum lunas"
			if x.Status == "paid" {
				status = "Lunas"
			}
			var due any = ""
			if x.DueDate != nil {
				if t, err := time.Parse("2006-01-02", *x.DueDate); err == nil {
					due = excelDate(t)
				}
			}
			late := "Tidak"
			if x.Overdue {
				late = "Ya"
			}
			rows = append(rows, []any{
				kindLabel[x.Kind], x.Counterparty, round2(x.Principal), round2(x.PaidAmount), round2(x.Remaining),
				status, due, late, x.Note, excelDateTime(x.CreatedAt),
			})
			overdue = append(overdue, x.Overdue)
		}
	}
	st := sh.st
	sh.section("Posisi saat ini", 10)
	first, _ := sh.table("PosisiHutang", []column{
		{"Jenis", 14, st.text, ""}, {"Pihak", 22, st.text, ""}, {"Pokok", 16, st.money, "sum"}, {"Terbayar", 16, st.money, "sum"},
		{"Sisa", 16, st.money, "sum"}, {"Status", 13, st.text, ""}, {"Jatuh tempo", 13, st.date, ""},
		{"Lewat tempo", 11, st.text, ""}, {"Catatan", 28, st.wrap, ""}, {"Dicatat pada", 18, st.datetime, ""},
	}, rows, false)
	for i, late := range overdue {
		if late && sh.err == nil {
			sh.err = sh.f.SetCellStyle(sh.name, cell(8, first+i), cell(8, first+i), st.bad)
		}
	}

	sh.section("Pergerakan dalam periode", 10)
	sh.note(period+". Efek ke saldo kosong berarti dicatat tanpa mengubah saldo dompet.", 10)
	sh.row++
	moveLabel := map[string]string{"principal": "Pinjaman awal", "payment": "Pembayaran"}
	var moves [][]any
	for _, m := range d.DebtMoves {
		wallet := ""
		if m.WalletName != nil {
			wallet = *m.WalletName
		}
		var effect any = ""
		if wallet != "" {
			effect = round2(m.WalletDelta)
		}
		moves = append(moves, []any{excelDate(m.Date), kindLabel[m.DebtKind], m.Counterparty, moveLabel[m.Kind], round2(m.Amount), wallet, effect, m.Note})
	}
	sh.table("PergerakanHutang", []column{
		{"Tanggal", 13, st.date, ""}, {"Jenis", 14, st.text, ""}, {"Pihak", 22, st.text, ""}, {"Pergerakan", 16, st.text, ""},
		{"Nominal", 16, st.money, "sum"}, {"Dompet", 20, st.text, ""}, {"Efek ke saldo", 16, st.money, "sum"}, {"Catatan", 28, st.wrap, ""},
	}, moves, false)
	sh.layout(true)
}

func renderGoals(sh *sheet, d *reportData) {
	sh.heading("Target Tabungan", "Kondisi per hari laporan dibuat. Progres dihitung dari saldo dompet yang dipakai target.")
	var rows [][]any
	for _, g := range d.Goals {
		var deadline, months, monthly any = "", "", ""
		if g.Deadline != nil {
			if t, err := time.Parse("2006-01-02", *g.Deadline); err == nil {
				deadline = excelDate(t)
			}
		}
		if g.MonthsLeft != nil {
			months = *g.MonthsLeft
		}
		if g.MonthlyNeeded != nil {
			monthly = round2(*g.MonthlyNeeded)
		}
		status := "Berjalan"
		if g.Achieved {
			status = "Tercapai"
		}
		rows = append(rows, []any{g.Name, g.Wallet.Name, round2(g.TargetAmount), round2(g.CurrentAmount), round2(g.Remaining), g.ProgressPct / 100, deadline, months, monthly, status})
	}
	st := sh.st
	first, _ := sh.table("TargetTabungan", []column{
		{"Target", 24, st.text, ""}, {"Dompet", 20, st.text, ""}, {"Nominal target", 16, st.money, "sum"},
		{"Terkumpul", 16, st.money, "sum"}, {"Kurang", 16, st.money, "sum"}, {"Progres", 11, st.pct, ""},
		{"Tenggat", 13, st.date, ""}, {"Bulan tersisa", 11, st.num, ""}, {"Setoran per bulan", 17, st.money, ""}, {"Status", 12, st.text, ""},
	}, rows, false)
	if len(rows) > 0 && sh.err == nil {
		sh.err = sh.f.SetConditionalFormat(sh.name, fmt.Sprintf("F%d:F%d", first, first+len(rows)-1), []excelize.ConditionalFormatOptions{{
			Type: "data_bar", Criteria: "=", MinType: "num", MinValue: "0", MaxType: "num", MaxValue: "1", BarColor: "#B9A6F5", BarSolid: true,
		}})
	}
	sh.layout(true)
}

func renderRecurring(sh *sheet, d *reportData) {
	sh.heading("Transaksi Rutin", "Jadwal per hari laporan dibuat. Transaksi yang sudah dicatat otomatis ada di sheet Transaksi dengan sumber Transaksi rutin.")
	freq := map[string]string{"weekly": "Mingguan", "monthly": "Bulanan"}
	var rows [][]any
	for _, r := range d.Recurring {
		parse := func(s string) any {
			if t, err := time.Parse("2006-01-02", s); err == nil {
				return excelDate(t)
			}
			return ""
		}
		var end any = ""
		if r.EndDate != nil {
			end = parse(*r.EndDate)
		}
		status := "Aktif"
		if !r.IsActive {
			status = "Dijeda"
		}
		perMonth := r.Amount
		if r.Frequency == "weekly" {
			perMonth = r.Amount * 52 / 12
		}
		rows = append(rows, []any{
			r.Note, entryKindLabel[r.Type], r.Category.Name, r.Wallet.Name, round2(r.Amount), freq[r.Frequency],
			round2(perMonth), parse(r.StartDate), end, parse(r.NextRunDate), status, r.LastError,
		})
	}
	st := sh.st
	sh.table("TransaksiRutin", []column{
		{"Catatan", 24, st.text, ""}, {"Jenis", 13, st.text, ""}, {"Kategori", 18, st.text, ""}, {"Dompet", 18, st.text, ""},
		{"Nominal", 15, st.money, ""}, {"Frekuensi", 11, st.text, ""}, {"Setara per bulan", 16, st.money, ""},
		{"Mulai", 13, st.date, ""}, {"Berakhir", 13, st.date, ""}, {"Berikutnya", 13, st.date, ""},
		{"Status", 10, st.text, ""}, {"Kendala terakhir", 28, st.wrap, ""},
	}, rows, false)
	sh.layout(true)
}
