package integration_test

import (
	"net/http"
	"strings"
	"testing"
)

// Aplikasi mengirim awal/akhir hari WIB dalam UTC, mis. 22 Okt 00:00 WIB =
// 2026-10-21T17:00:00Z. Transaksi tanggal 21 tidak boleh ikut terambil.
const wibPeriod = "?start_date=2026-10-21T17:00:00.000Z&end_date=2026-11-21T16:59:59.999Z"

func TestDateFilterUsesWIBCalendar(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 10_000_000)
	cash := createWallet(t, u, "Tunai", 0)
	salary := createCategory(t, u, "Gaji", "income")

	for _, day := range []string{"2026-10-21", "2026-10-22", "2026-11-21", "2026-11-22"} {
		incomeOn(t, u, bca, salary, 1_000, day)
		transferOn(t, u, bca, cash, 1_000, 0, day)
	}

	status, r := call(t, http.MethodGet, "/api/transactions"+wibPeriod, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	txDates := map[string]bool{}
	for _, tx := range decode[[]struct {
		Date string `json:"date"`
	}](t, r) {
		txDates[tx.Date[:10]] = true
	}
	if len(txDates) != 2 || !txDates["2026-10-22"] || !txDates["2026-11-21"] {
		t.Fatalf("tanggal transaksi %v, ingin hanya 22 Okt dan 21 Nov", txDates)
	}

	status, r = call(t, http.MethodGet, "/api/transfers"+wibPeriod, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if n := len(decode[[]struct {
		ID string `json:"id"`
	}](t, r)); n != 2 {
		t.Fatalf("%d pindahan, ingin 2 (22 Okt dan 21 Nov)", n)
	}

	res, rows := fetchCSV(t, u, "/api/export/transactions.csv"+wibPeriod)
	if len(rows) != 3 || rows[1][0] != "2026-10-22" || rows[2][0] != "2026-11-21" {
		t.Fatalf("CSV %v, ingin header + 22 Okt + 21 Nov", rows)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, "transaksi_20261022_20261121.csv") {
		t.Fatalf("nama file CSV %q", cd)
	}
}
