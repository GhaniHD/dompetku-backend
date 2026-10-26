package integration_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func downloadXLSX(t *testing.T, u testUser, query string) (*excelize.File, *http.Response) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/export/report.xlsx"+query, nil)
	req.Header.Set("Authorization", "Bearer "+u.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, res
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("file bukan xlsx valid: %v", err)
	}
	return f, res
}

// tableRows mengembalikan baris data sebuah Excel Table sebagai map judul kolom → nilai mentah.
func tableRows(t *testing.T, f *excelize.File, sheet, table string) []map[string]string {
	t.Helper()
	tables, err := f.GetTables(sheet)
	if err != nil {
		t.Fatalf("tabel %s: %v", sheet, err)
	}
	for _, tb := range tables {
		if tb.Name != table {
			continue
		}
		parts := strings.Split(tb.Range, ":")
		c1, r1, _ := excelize.CellNameToCoordinates(parts[0])
		c2, r2, _ := excelize.CellNameToCoordinates(parts[1])
		var headers []string
		for c := c1; c <= c2; c++ {
			name, _ := excelize.CoordinatesToCellName(c, r1)
			v, _ := f.GetCellValue(sheet, name)
			headers = append(headers, v)
		}
		var out []map[string]string
		for r := r1 + 1; r <= r2; r++ {
			row := map[string]string{}
			for i, h := range headers {
				name, _ := excelize.CoordinatesToCellName(c1+i, r)
				v, _ := f.GetCellValue(sheet, name, excelize.Options{RawCellValue: true})
				row[h] = v
			}
			out = append(out, row)
		}
		return out
	}
	t.Fatalf("tabel %s tidak ada di sheet %s", table, sheet)
	return nil
}

func TestExportXLSXCoversEveryMovement(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 1_000_000)
	cash := createWallet(t, u, "Tunai", 0)
	salary := createCategory(t, u, "Gaji", "income")
	food := createCategory(t, u, "Makan", "expense")

	expenseOn(t, u, bca, food, 50_000, "2026-09-30")
	incomeOn(t, u, bca, salary, 5_000_000, "2026-10-01")
	expenseOn(t, u, bca, food, 75_000, "2026-10-02")
	transferOn(t, u, bca, cash, 200_000, 2_500, "2026-10-03")
	expenseOn(t, u, cash, food, 30_000, "2026-10-04")
	status, r := call(t, http.MethodPost, "/api/wallets/"+cash+"/adjust", u.Token, map[string]any{
		"actual_balance": 165_000, "date": "2026-10-05T09:00:00+07:00",
	})
	expectStatus(t, status, http.StatusCreated, r)
	status, r = call(t, http.MethodPost, "/api/debts", u.Token, map[string]any{
		"kind": "receivable", "counterparty": "Rizky", "amount": 100_000, "wallet_id": bca, "date": "2026-10-06T09:00:00+07:00",
	})
	expectStatus(t, status, http.StatusCreated, r)
	expenseOn(t, u, bca, food, 99_000, "2026-11-01")

	f, _ := downloadXLSX(t, u, "?start=2026-10-01&end=2026-10-31")
	defer f.Close()

	want := []string{"Ringkasan", "Transaksi", "Mutasi Dompet", "Pindahan", "Per Kategori", "Harian", "Bulanan", "Anggaran", "Hutang & Piutang", "Target Tabungan", "Transaksi Rutin"}
	if got := f.GetSheetList(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("sheet %v, ingin %v", got, want)
	}

	// Transaksi: hanya rentang Oktober, termasuk biaya admin transfer.
	txs := tableRows(t, f, "Transaksi", "Transaksi")
	if len(txs) != 4 {
		t.Fatalf("%d transaksi di Excel, ingin 4 (gaji, makan, biaya admin, makan tunai)", len(txs))
	}
	fee := 0
	for _, row := range txs {
		if row["Sumber"] == "Biaya admin transfer" {
			fee++
		}
	}
	if fee != 1 {
		t.Fatalf("biaya admin transfer tercatat %d kali, ingin 1", fee)
	}

	// Posisi dompet: saldo awal = sebelum 1 Okt, saldo akhir = saldo per 31 Okt.
	positions := map[string]map[string]string{}
	for _, row := range tableRows(t, f, "Ringkasan", "PosisiDompet") {
		positions[row["Dompet"]] = row
	}
	expectCell := func(wallet, col, want string) {
		t.Helper()
		if got := positions[wallet][col]; got != want {
			t.Fatalf("%s kolom %q = %q, ingin %q", wallet, col, got, want)
		}
	}
	expectCell("BCA", "Saldo awal", "950000")
	expectCell("BCA", "Saldo akhir", "5572500")
	expectCell("BCA", "Hutang & piutang", "-100000")
	expectCell("Tunai", "Saldo awal", "0")
	expectCell("Tunai", "Penyesuaian", "-5000")
	expectCell("Tunai", "Saldo akhir", "165000")

	// Saldo akhir tiap dompet + transaksi November = saldo sekarang.
	if got := balanceOf(t, u, bca); got != 5_572_500-99_000 {
		t.Fatalf("saldo BCA sekarang %.0f, ingin %d", got, 5_572_500-99_000)
	}

	// Mutasi: baris terakhir tiap dompet sama dengan saldo akhir di ringkasan.
	last := map[string]string{}
	for _, row := range tableRows(t, f, "Mutasi Dompet", "MutasiDompet") {
		last[row["Dompet"]] = row["Saldo setelahnya"]
	}
	if last["BCA"] != "5572500" || last["Tunai"] != "165000" {
		t.Fatalf("saldo akhir mutasi %v tidak cocok dengan ringkasan", last)
	}

	if moves := tableRows(t, f, "Hutang & Piutang", "PergerakanHutang"); len(moves) != 1 || moves[0]["Pihak"] != "Rizky" {
		t.Fatalf("pergerakan hutang %v, ingin 1 baris Rizky", moves)
	}
	if days := tableRows(t, f, "Harian", "Harian"); len(days) != 31 {
		t.Fatalf("%d hari di sheet Harian, ingin 31", len(days))
	}
}

func TestExportXLSXValidation(t *testing.T) {
	u := newUser(t)
	for query, want := range map[string]int{
		"?start=2026-10-10&end=2026-10-01": http.StatusBadRequest,
		"?start=2020-01-01&end=2026-10-01": http.StatusBadRequest,
		"?start=10-01-2026":                http.StatusBadRequest,
		"":                                 http.StatusOK,
	} {
		f, res := downloadXLSX(t, u, query)
		if res.StatusCode != want {
			t.Fatalf("%q status %d, ingin %d", query, res.StatusCode, want)
		}
		if f != nil {
			if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "spreadsheetml") {
				t.Fatalf("content-type %q", ct)
			}
			f.Close()
		}
	}

	other := newUser(t)
	createWallet(t, other, "Rahasia", 9_999_999)
	f, _ := downloadXLSX(t, u, "")
	defer f.Close()
	for _, row := range tableRows(t, f, "Ringkasan", "PosisiDompet") {
		if row["Dompet"] == "Rahasia" {
			t.Fatal("data dompet user lain ikut terekspor")
		}
	}
}

func downloadSingle(t *testing.T, u testUser, path, query string) *excelize.File {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/export/"+path+query, nil)
	req.Header.Set("Authorization", "Bearer "+u.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s status %d: %s", path, res.StatusCode, raw)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, ".xlsx") {
		t.Fatalf("content-disposition %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("file bukan xlsx valid: %v", err)
	}
	return f
}

func TestExportSingleSheets(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 1_000_000)
	cash := createWallet(t, u, "Tunai", 0)
	salary := createCategory(t, u, "Gaji", "income")
	food := createCategory(t, u, "Makan", "expense")

	incomeOn(t, u, bca, salary, 5_000_000, "2026-10-01")
	expenseOn(t, u, bca, food, 75_000, "2026-10-02")
	transferOn(t, u, bca, cash, 200_000, 2_500, "2026-10-03")
	expenseOn(t, u, cash, food, 30_000, "2026-11-02")

	f := downloadSingle(t, u, "transactions.xlsx", "?start=2026-10-01&end=2026-10-31")
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 1 || got[0] != "Transaksi" {
		t.Fatalf("sheet %v, ingin hanya Transaksi", got)
	}
	txs := tableRows(t, f, "Transaksi", "Transaksi")
	if len(txs) != 3 {
		t.Fatalf("%d transaksi, ingin 3 (gaji, makan, biaya admin)", len(txs))
	}
	formula, _ := f.GetCellFormula("Transaksi", "C4")
	if !strings.Contains(formula, "SUBTOTAL(109,Transaksi[Pemasukan])") {
		t.Fatalf("ringkasan pemasukan bukan SUBTOTAL: %q", formula)
	}

	g := downloadSingle(t, u, "transfers.xlsx", "?start=2026-10-01&end=2026-10-31")
	defer g.Close()
	moves := tableRows(t, g, "Pindahan", "Pindahan")
	if len(moves) != 1 || moves[0]["Dari dompet"] != "BCA" || moves[0]["Ke dompet"] != "Tunai" || moves[0]["Biaya admin"] != "2500" {
		t.Fatalf("pindahan %v", moves)
	}

	empty := downloadSingle(t, u, "transfers.xlsx", "?start=2026-12-01&end=2026-12-31")
	defer empty.Close()
	if rows := tableRows(t, empty, "Pindahan", "Pindahan"); len(rows) != 1 || rows[0]["No"] != "Tidak ada data pada periode ini" {
		t.Fatalf("periode kosong %v", rows)
	}

	other := newUser(t)
	o := downloadSingle(t, other, "transactions.xlsx", "?start=2026-10-01&end=2026-10-31")
	defer o.Close()
	if rows := tableRows(t, o, "Transaksi", "Transaksi"); len(rows) != 1 || rows[0]["No"] != "Tidak ada data pada periode ini" {
		t.Fatalf("transaksi user lain ikut terekspor: %v", rows)
	}
}
