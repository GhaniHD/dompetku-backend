package integration_test

import (
	"encoding/csv"
	"io"
	"net/http"
	"strings"
	"testing"
)

type historyResult struct {
	OpeningBalance float64 `json:"opening_balance"`
	Total          int64   `json:"total"`
	Wallet         struct {
		Balance float64 `json:"balance"`
	} `json:"wallet"`
	Entries []struct {
		Kind         string  `json:"kind"`
		Amount       float64 `json:"amount"`
		Date         string  `json:"date"`
		CategoryName string  `json:"category_name"`
		Counterpart  string  `json:"counterpart"`
		BalanceAfter float64 `json:"balance_after"`
	} `json:"entries"`
}

func walletHistory(t *testing.T, u testUser, walletID, query string) historyResult {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/wallets/"+walletID+"/history"+query, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[historyResult](t, r)
}

func incomeOn(t *testing.T, u testUser, walletID, categoryID string, amount float64, date string) {
	t.Helper()
	body := txBody(walletID, categoryID, amount, "income")
	body["date"] = date + "T09:00:00+07:00"
	status, r := call(t, http.MethodPost, "/api/transactions", u.Token, body)
	expectStatus(t, status, http.StatusCreated, r)
}

func transferOn(t *testing.T, u testUser, from, to string, amount, fee float64, date string) {
	t.Helper()
	body := transferBody(from, to, amount, fee)
	body["date"] = date + "T09:00:00+07:00"
	status, r := call(t, http.MethodPost, "/api/transfers", u.Token, body)
	expectStatus(t, status, http.StatusCreated, r)
}

func TestWalletHistoryRunningBalance(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 100_000)
	cash := createWallet(t, u, "Cash", 50_000)
	salary := createCategory(t, u, "Gaji", "income")
	food := createCategory(t, u, "Makan", "expense")

	incomeOn(t, u, bca, salary, 500_000, "2026-10-01")
	expenseOn(t, u, bca, food, 20_000, "2026-10-02")
	transferOn(t, u, bca, cash, 30_000, 1_000, "2026-10-03")
	transferOn(t, u, cash, bca, 10_000, 0, "2026-10-04")
	status, r := call(t, http.MethodPost, "/api/wallets/"+bca+"/adjust", u.Token, map[string]any{
		"actual_balance": 550_000, "date": "2026-10-05T09:00:00+07:00",
	})
	expectStatus(t, status, http.StatusCreated, r)

	h := walletHistory(t, u, bca, "?limit=100")

	// Biaya admin dibuat sesaat sebelum transfernya, jadi tampil tepat di bawah transfer.
	wantKinds := []string{"adjustment", "transfer_in", "transfer_out", "expense", "expense", "income"}
	if len(h.Entries) != len(wantKinds) || h.Total != int64(len(wantKinds)) {
		t.Fatalf("jumlah entri %d (total %d), ingin %d", len(h.Entries), h.Total, len(wantKinds))
	}
	for i, k := range wantKinds {
		if h.Entries[i].Kind != k {
			t.Fatalf("entri %d bertipe %q, ingin %q", i, h.Entries[i].Kind, k)
		}
	}
	if h.Entries[0].BalanceAfter != h.Wallet.Balance || h.Wallet.Balance != 550_000 {
		t.Fatalf("saldo setelah entri terbaru %.0f, saldo dompet %.0f, ingin 550000", h.Entries[0].BalanceAfter, h.Wallet.Balance)
	}
	for i := 0; i < len(h.Entries)-1; i++ {
		if got := h.Entries[i].BalanceAfter - h.Entries[i].Amount; got != h.Entries[i+1].BalanceAfter {
			t.Fatalf("saldo berjalan putus di entri %d: %.0f vs %.0f", i, got, h.Entries[i+1].BalanceAfter)
		}
	}
	oldest := h.Entries[len(h.Entries)-1]
	if oldest.BalanceAfter-oldest.Amount != h.OpeningBalance {
		t.Fatalf("entri tertua tidak berawal dari saldo awal %.0f", h.OpeningBalance)
	}
	if h.Entries[2].Counterpart != "Cash" || h.Entries[3].CategoryName != "Biaya Admin" {
		t.Fatalf("detail transfer/biaya admin salah: %+v / %+v", h.Entries[2], h.Entries[3])
	}

	page := walletHistory(t, u, bca, "?page=2&limit=2")
	if len(page.Entries) != 2 || page.Entries[0].Kind != "transfer_out" {
		t.Fatalf("pagination riwayat salah: %+v", page.Entries)
	}

	ranged := walletHistory(t, u, bca, "?start_date=2026-10-02T00:00:00Z&end_date=2026-10-03T00:00:00Z")
	if ranged.Total != 3 {
		t.Fatalf("filter tanggal 2–3 Okt: %d entri, ingin 3", ranged.Total)
	}

	other := newUser(t)
	status, r = call(t, http.MethodGet, "/api/wallets/"+bca+"/history", other.Token, nil)
	expectStatus(t, status, http.StatusNotFound, r)
}

func TestMonthlyReportCyclePeriod(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "BCA", 10_000_000)
	food := createCategory(t, u, "Makan", "expense")
	setBudgetStartDay(t, u, 25)

	expenseOn(t, u, wallet, food, 100_000, "2026-10-24")
	expenseOn(t, u, wallet, food, 200_000, "2026-10-26")
	expenseOn(t, u, wallet, food, 400_000, "2026-11-20")

	type report struct {
		Period       string  `json:"period"`
		PeriodStart  string  `json:"period_start"`
		PeriodEnd    string  `json:"period_end"`
		TotalExpense float64 `json:"total_expense"`
	}

	status, r := call(t, http.MethodGet, "/api/reports?month=10&year=2026", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	cal := decode[report](t, r)
	if cal.Period != "calendar" || cal.PeriodStart != "2026-10-01" || cal.PeriodEnd != "2026-10-31" || cal.TotalExpense != 300_000 {
		t.Fatalf("laporan kalender salah: %+v", cal)
	}

	status, r = call(t, http.MethodGet, "/api/reports?month=10&year=2026&period=cycle", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	cyc := decode[report](t, r)
	if cyc.Period != "cycle" || cyc.PeriodStart != "2026-10-25" || cyc.PeriodEnd != "2026-11-24" || cyc.TotalExpense != 600_000 {
		t.Fatalf("laporan siklus gajian salah: %+v", cyc)
	}

	status, r = call(t, http.MethodGet, "/api/reports?month=10&year=2026&period=mingguan", u.Token, nil)
	expectStatus(t, status, http.StatusBadRequest, r)
}

func fetchCSV(t *testing.T, u testUser, path string) (*http.Response, [][]string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+u.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if !strings.HasPrefix(string(raw), "\xEF\xBB\xBF") {
		t.Fatal("CSV tidak diawali BOM UTF-8, Excel bisa salah membaca huruf")
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\xEF\xBB\xBF"))).ReadAll()
	if err != nil {
		t.Fatalf("CSV tidak valid: %v", err)
	}
	return res, records
}

func TestExportCSV(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 1_000_000)
	cash := createWallet(t, u, "Cash", 0)
	food := createCategory(t, u, "Makan, Minum", "expense")
	expenseOn(t, u, bca, food, 15_000, "2026-10-02")
	expenseOn(t, u, bca, food, 25_500.5, "2026-10-01")
	transferOn(t, u, bca, cash, 100_000, 2_500, "2026-10-03")

	res, rows := fetchCSV(t, u, "/api/export/transactions.csv?start_date=2026-10-01T00:00:00Z&end_date=2026-10-31T00:00:00Z")
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type %q", ct)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, "transaksi_20261001_20261031.csv") {
		t.Fatalf("nama file %q", cd)
	}
	want := [][]string{
		{"tanggal", "tipe", "kategori", "dompet", "nominal", "catatan"},
		{"2026-10-01", "expense", "Makan, Minum", "BCA", "25500.5", ""},
		{"2026-10-02", "expense", "Makan, Minum", "BCA", "15000", ""},
		{"2026-10-03", "expense", "Biaya Admin", "BCA", "2500", "Biaya admin transfer BCA → Cash"},
	}
	if len(rows) != len(want) {
		t.Fatalf("CSV transaksi %d baris, ingin %d: %v", len(rows), len(want), rows)
	}
	for i := range want {
		if strings.Join(rows[i], "|") != strings.Join(want[i], "|") {
			t.Fatalf("baris %d = %v, ingin %v", i, rows[i], want[i])
		}
	}

	_, rows = fetchCSV(t, u, "/api/export/transfers.csv")
	if len(rows) != 2 || strings.Join(rows[1], "|") != "2026-10-03|BCA|Cash|100000|2500|tes" {
		t.Fatalf("CSV transfer salah: %v", rows)
	}

	other := newUser(t)
	_, rows = fetchCSV(t, other, "/api/export/transactions.csv")
	if len(rows) != 1 {
		t.Fatalf("user lain melihat %d baris transaksi orang lain", len(rows)-1)
	}
}
