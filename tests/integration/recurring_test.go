package integration_test

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

type recurringResult struct {
	ID          string  `json:"id"`
	NextRunDate string  `json:"next_run_date"`
	IsActive    bool    `json:"is_active"`
	LastError   string  `json:"last_error"`
	Amount      float64 `json:"amount"`
}

var wib = time.FixedZone("WIB", 7*60*60)

func todayWIB() time.Time {
	n := time.Now().In(wib)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func createRecurring(t *testing.T, u testUser, body map[string]any) recurringResult {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/recurring", u.Token, body)
	expectStatus(t, status, http.StatusCreated, r)
	return decode[recurringResult](t, r)
}

func recurringBody(walletID, categoryID, kind, freq string, amount float64, start time.Time) map[string]any {
	return map[string]any{
		"wallet_id": walletID, "category_id": categoryID, "type": kind, "frequency": freq,
		"amount": amount, "note": "otomatis", "start_date": start.Format("2006-01-02") + "T00:00:00+07:00",
	}
}

func countRecurringTx(t *testing.T, ruleID string) int64 {
	t.Helper()
	return countRows(t, "SELECT COUNT(*) FROM transactions WHERE recurring_id = ? AND deleted_at IS NULL", ruleID)
}

func monthlyOccurrences(start, until time.Time) int {
	n := 0
	for d := start; !d.After(until); d = d.AddDate(0, 1, 0) {
		n++
	}
	return n
}

func TestRecurringCatchUpWithoutDuplicates(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 0)
	salary := createCategory(t, u, "Gaji", "income")

	today := todayWIB()
	start := time.Date(today.Year(), today.Month()-2, 1, 0, 0, 0, 0, time.UTC)
	rule := createRecurring(t, u, recurringBody(bca, salary, "income", "monthly", 5_000_000, start))

	if got := countRecurringTx(t, rule.ID); got != 3 {
		t.Fatalf("jadwal tanggal 1 sejak 2 bulan lalu: %d transaksi tercatat, ingin 3", got)
	}
	expectBalance(t, u, bca, 15_000_000)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := recurringSvc.ProcessDue(time.Now()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("ProcessDue paralel gagal: %v", err)
	}
	if got := countRecurringTx(t, rule.ID); got != 3 {
		t.Fatalf("ProcessDue paralel membuat duplikat: %d transaksi", got)
	}

	future := time.Now().AddDate(0, 0, 40)
	if _, err := recurringSvc.ProcessDue(future); err != nil {
		t.Fatalf("ProcessDue: %v", err)
	}
	want := monthlyOccurrences(start, todayWIB().AddDate(0, 0, 40))
	if got := countRecurringTx(t, rule.ID); got != int64(want) {
		t.Fatalf("setelah 40 hari: %d transaksi, ingin %d", got, want)
	}
	expectReconciled(t, u)

	status, r := call(t, http.MethodGet, "/api/transactions", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	for _, tx := range decode[[]struct {
		RecurringID *string `json:"recurring_id"`
	}](t, r) {
		if tx.RecurringID == nil || *tx.RecurringID != rule.ID {
			t.Fatal("transaksi otomatis tidak membawa recurring_id")
		}
	}
}

func TestRecurringStopsAtEndDate(t *testing.T) {
	u := newUser(t)
	cash := createWallet(t, u, "Cash", 1_000_000)
	snack := createCategory(t, u, "Langganan", "expense")

	start := todayWIB().AddDate(0, 0, -21)
	body := recurringBody(cash, snack, "expense", "weekly", 10_000, start)
	body["end_date"] = todayWIB().AddDate(0, 0, -7).Format("2006-01-02") + "T00:00:00+07:00"
	rule := createRecurring(t, u, body)

	if got := countRecurringTx(t, rule.ID); got != 3 {
		t.Fatalf("mingguan 3 minggu lalu s.d. 1 minggu lalu: %d transaksi, ingin 3", got)
	}
	if rule.IsActive {
		t.Fatal("jadwal yang melewati tanggal akhir harus nonaktif")
	}
	expectBalance(t, u, cash, 970_000)
}

func TestRecurringFailureAdvancesAndNotifies(t *testing.T) {
	u := newUser(t)
	cash := createWallet(t, u, "Cash", 10_000)
	rent := createCategory(t, u, "Kos", "expense")

	rule := createRecurring(t, u, recurringBody(cash, rent, "expense", "monthly", 50_000, todayWIB()))

	if got := countRecurringTx(t, rule.ID); got != 0 {
		t.Fatalf("transaksi tercatat padahal saldo tidak cukup: %d", got)
	}
	if rule.LastError == "" || rule.NextRunDate <= todayWIB().Format("2006-01-02") {
		t.Fatalf("jadwal harus dimajukan dan mencatat error: %+v", rule)
	}
	if got := countTitles(t, u, "Transaksi Berulang Gagal"); got != 1 {
		t.Fatalf("notifikasi gagal %d, ingin 1", got)
	}
	expectBalance(t, u, cash, 10_000)
	expectReconciled(t, u)
}

func TestRecurringPauseDoesNotBackfill(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 0)
	salary := createCategory(t, u, "Gaji", "income")
	rule := createRecurring(t, u, recurringBody(bca, salary, "income", "monthly", 1_000_000, todayWIB()))

	status, r := call(t, http.MethodPut, "/api/recurring/"+rule.ID, u.Token, map[string]any{"is_active": false})
	expectStatus(t, status, http.StatusOK, r)
	if _, err := recurringSvc.ProcessDue(time.Now().AddDate(0, 3, 0)); err != nil {
		t.Fatalf("ProcessDue: %v", err)
	}
	if got := countRecurringTx(t, rule.ID); got != 1 {
		t.Fatalf("jadwal yang dijeda tetap mencatat: %d transaksi", got)
	}

	if err := gdb.Exec("UPDATE recurring_transactions SET next_run_date = next_run_date - INTERVAL '120 days' WHERE id = ?", rule.ID).Error; err != nil {
		t.Fatal(err)
	}
	status, r = call(t, http.MethodPut, "/api/recurring/"+rule.ID, u.Token, map[string]any{"is_active": true})
	expectStatus(t, status, http.StatusOK, r)
	resumed := decode[recurringResult](t, r)

	if got := countRecurringTx(t, rule.ID); got > 2 {
		t.Fatalf("mengaktifkan lagi mencatat mundur jadwal yang terlewat: %d transaksi", got)
	}
	if resumed.NextRunDate < todayWIB().Format("2006-01-02") {
		t.Fatalf("jadwal berikutnya masih di masa lalu: %s", resumed.NextRunDate)
	}
}

func TestRecurringValidationAndOwnership(t *testing.T) {
	owner := newUser(t)
	wallet := createWallet(t, owner, "Cash", 100_000)
	food := createCategory(t, owner, "Makan", "expense")
	salary := createCategory(t, owner, "Gaji", "income")
	rule := createRecurring(t, owner, recurringBody(wallet, food, "expense", "weekly", 1_000, todayWIB().AddDate(0, 0, 3)))

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"kategori tidak cocok dengan tipe", recurringBody(wallet, salary, "expense", "monthly", 1_000, todayWIB()), http.StatusBadRequest},
		{"frekuensi tidak dikenal", recurringBody(wallet, food, "expense", "daily", 1_000, todayWIB()), http.StatusBadRequest},
		{"nominal nol", recurringBody(wallet, food, "expense", "monthly", 0, todayWIB()), http.StatusBadRequest},
	}
	endBefore := recurringBody(wallet, food, "expense", "monthly", 1_000, todayWIB())
	endBefore["end_date"] = todayWIB().AddDate(0, 0, -1).Format("2006-01-02") + "T00:00:00+07:00"
	cases = append(cases, struct {
		name string
		body map[string]any
		want int
	}{"tanggal akhir sebelum mulai", endBefore, http.StatusBadRequest})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, r := call(t, http.MethodPost, "/api/recurring", owner.Token, tc.body)
			expectStatus(t, status, tc.want, r)
		})
	}

	other := newUser(t)
	otherFood := createCategory(t, other, "Jajan", "expense")
	status, r := call(t, http.MethodPost, "/api/recurring", other.Token, recurringBody(wallet, otherFood, "expense", "monthly", 1_000, todayWIB()))
	expectStatus(t, status, http.StatusNotFound, r)

	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		status, r := call(t, m, "/api/recurring/"+rule.ID, other.Token, map[string]any{"amount": 1})
		expectStatus(t, status, http.StatusNotFound, r)
	}

	status, r = call(t, http.MethodDelete, "/api/recurring/"+rule.ID, owner.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if _, err := recurringSvc.ProcessDue(time.Now().AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if got := countRecurringTx(t, rule.ID); got != 0 {
		t.Fatalf("jadwal yang dihapus tetap mencatat %d transaksi", got)
	}
}

func TestRecurringParallelProcessingNoDuplicates(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 0)
	salary := createCategory(t, u, "Gaji", "income")

	today := todayWIB()
	future := time.Date(today.Year(), today.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	rule := createRecurring(t, u, recurringBody(bca, salary, "income", "monthly", 1_000_000, future))

	start := time.Date(today.Year(), today.Month()-2, 1, 0, 0, 0, 0, time.UTC)
	if err := gdb.Exec("UPDATE recurring_transactions SET start_date = ?, next_run_date = ? WHERE id = ?",
		start.Format("2006-01-02"), start.Format("2006-01-02"), rule.ID).Error; err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = recurringSvc.ProcessDue(time.Now())
		}()
	}
	wg.Wait()

	if got := countRecurringTx(t, rule.ID); got != 3 {
		t.Fatalf("10 proses paralel mencatat %d transaksi, ingin tepat 3", got)
	}
	expectBalance(t, u, bca, 3_000_000)
	expectReconciled(t, u)
}
