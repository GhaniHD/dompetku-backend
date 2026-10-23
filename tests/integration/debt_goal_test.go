package integration_test

import (
	"net/http"
	"testing"
	"time"
)

type debtResult struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Principal  float64 `json:"principal"`
	PaidAmount float64 `json:"paid_amount"`
	Remaining  float64 `json:"remaining"`
	Status     string  `json:"status"`
	Overdue    bool    `json:"overdue"`
	Movements  []struct {
		ID          string  `json:"id"`
		Kind        string  `json:"kind"`
		WalletDelta float64 `json:"wallet_delta"`
	} `json:"movements"`
}

func createDebt(t *testing.T, u testUser, body map[string]any) debtResult {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/debts", u.Token, body)
	expectStatus(t, status, http.StatusCreated, r)
	return decode[debtResult](t, r)
}

func payDebt(t *testing.T, u testUser, debtID string, body map[string]any) (int, apiResponse) {
	t.Helper()
	return call(t, http.MethodPost, "/api/debts/"+debtID+"/payments", u.Token, body)
}

func TestPayableDebtFlow(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 100_000)
	before := reportOctober(t, u)

	debt := createDebt(t, u, map[string]any{
		"kind": "payable", "counterparty": "Budi", "amount": 1_000_000,
		"wallet_id": bca, "date": testDate,
	})
	expectBalance(t, u, bca, 1_100_000)

	status, r := payDebt(t, u, debt.ID, map[string]any{"amount": 400_000, "wallet_id": bca, "date": testDate})
	expectStatus(t, status, http.StatusCreated, r)
	partial := decode[debtResult](t, r)
	if partial.Remaining != 600_000 || partial.Status != "open" || len(partial.Movements) != 2 {
		t.Fatalf("setelah cicilan pertama: %+v", partial)
	}
	expectBalance(t, u, bca, 700_000)

	t.Run("bayar melebihi sisa ditolak", func(t *testing.T) {
		status, r := payDebt(t, u, debt.ID, map[string]any{"amount": 600_001, "wallet_id": bca})
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	status, r = payDebt(t, u, debt.ID, map[string]any{"amount": 600_000, "wallet_id": bca, "date": testDate})
	expectStatus(t, status, http.StatusCreated, r)
	if paid := decode[debtResult](t, r); paid.Status != "paid" || paid.Remaining != 0 {
		t.Fatalf("hutang harus lunas: %+v", paid)
	}
	expectBalance(t, u, bca, 100_000)
	if got := countTitles(t, u, "Hutang Lunas"); got != 1 {
		t.Fatalf("notifikasi lunas %d, ingin 1", got)
	}

	if after := reportOctober(t, u); after.TotalIncome != before.TotalIncome || after.TotalExpense != before.TotalExpense {
		t.Fatal("hutang dan cicilannya tidak boleh masuk laporan pemasukan/pengeluaran")
	}

	h := walletHistory(t, u, bca, "?limit=100")
	if h.Total != 3 || h.Entries[0].Kind != "debt_payable" || h.Entries[0].Counterpart != "Budi" {
		t.Fatalf("riwayat dompet tidak memuat pergerakan hutang: %+v", h.Entries)
	}
	if h.Entries[0].BalanceAfter != 100_000 {
		t.Fatalf("saldo berjalan riwayat %.0f, ingin 100000", h.Entries[0].BalanceAfter)
	}
	expectReconciled(t, u)
}

func TestReceivableDebtAndBalanceRules(t *testing.T) {
	u := newUser(t)
	cash := createWallet(t, u, "Cash", 300_000)

	t.Run("meminjamkan melebihi saldo ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/debts", u.Token, map[string]any{
			"kind": "receivable", "counterparty": "Andi", "amount": 500_000, "wallet_id": cash,
		})
		expectStatus(t, status, http.StatusBadRequest, r)
		if n := countRows(t, "SELECT COUNT(*) FROM debts WHERE user_id = ?", u.ID); n != 0 {
			t.Fatalf("hutang tersimpan walau saldo tidak cukup: %d", n)
		}
	})

	debt := createDebt(t, u, map[string]any{
		"kind": "receivable", "counterparty": "Andi", "amount": 200_000, "wallet_id": cash,
	})
	expectBalance(t, u, cash, 100_000)

	status, r := payDebt(t, u, debt.ID, map[string]any{"amount": 50_000, "wallet_id": cash})
	expectStatus(t, status, http.StatusCreated, r)
	expectBalance(t, u, cash, 150_000)

	paymentID := ""
	for _, m := range decode[debtResult](t, r).Movements {
		if m.Kind == "payment" {
			paymentID = m.ID
		}
	}
	status, r = call(t, http.MethodDelete, "/api/debts/"+debt.ID+"/payments/"+paymentID, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if d := decode[debtResult](t, r); d.PaidAmount != 0 || d.Remaining != 200_000 {
		t.Fatalf("pembatalan pembayaran salah: %+v", d)
	}
	expectBalance(t, u, cash, 100_000)

	status, r = call(t, http.MethodDelete, "/api/debts/"+debt.ID, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	expectBalance(t, u, cash, 300_000)
	expectReconciled(t, u)
}

func TestDebtWithoutWalletAndSummary(t *testing.T) {
	u := newUser(t)
	createDebt(t, u, map[string]any{"kind": "payable", "counterparty": "Koperasi", "amount": 2_000_000})
	createDebt(t, u, map[string]any{"kind": "receivable", "counterparty": "Rina", "amount": 150_000})
	paid := createDebt(t, u, map[string]any{"kind": "receivable", "counterparty": "Dodi", "amount": 50_000})
	status, r := payDebt(t, u, paid.ID, map[string]any{"amount": 50_000})
	expectStatus(t, status, http.StatusCreated, r)

	status, r = call(t, http.MethodGet, "/api/debts", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	summary := decode[struct {
		TotalPayable    float64      `json:"total_payable"`
		TotalReceivable float64      `json:"total_receivable"`
		Debts           []debtResult `json:"debts"`
	}](t, r)
	if summary.TotalPayable != 2_000_000 || summary.TotalReceivable != 150_000 || len(summary.Debts) != 3 {
		t.Fatalf("ringkasan salah: %+v", summary)
	}

	status, r = call(t, http.MethodGet, "/api/debts?status=open&kind=receivable", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if open := decode[struct {
		Debts []debtResult `json:"debts"`
	}](t, r).Debts; len(open) != 1 {
		t.Fatalf("filter piutang belum lunas: %d, ingin 1", len(open))
	}
}

func TestDebtDueReminderOncePerDay(t *testing.T) {
	u := newUser(t)
	tomorrow := todayWIB().AddDate(0, 0, 1).Format("2006-01-02") + "T00:00:00+07:00"
	createDebt(t, u, map[string]any{"kind": "payable", "counterparty": "Bank", "amount": 500_000, "due_date": tomorrow})
	nextWeek := todayWIB().AddDate(0, 0, 7).Format("2006-01-02") + "T00:00:00+07:00"
	createDebt(t, u, map[string]any{"kind": "payable", "counterparty": "Nanti", "amount": 500_000, "due_date": nextWeek})

	for i := 0; i < 3; i++ {
		if _, err := debtSvc.SendDueReminders(time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if got := countTitles(t, u, "Hutang Jatuh Tempo"); got != 1 {
		t.Fatalf("pengingat H-1 terkirim %d kali, ingin 1", got)
	}

	if _, err := debtSvc.SendDueReminders(time.Now().AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if got := countTitles(t, u, "Hutang Jatuh Tempo"); got != 2 {
		t.Fatalf("pengingat hari H: total %d, ingin 2", got)
	}
}

func TestDebtOwnership(t *testing.T) {
	owner := newUser(t)
	debt := createDebt(t, owner, map[string]any{"kind": "payable", "counterparty": "Budi", "amount": 100_000})
	other := newUser(t)
	otherWallet := createWallet(t, other, "Cash", 1_000_000)

	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		status, r := call(t, m, "/api/debts/"+debt.ID, other.Token, map[string]any{"note": "x"})
		expectStatus(t, status, http.StatusNotFound, r)
	}
	status, r := payDebt(t, other, debt.ID, map[string]any{"amount": 1_000, "wallet_id": otherWallet})
	expectStatus(t, status, http.StatusNotFound, r)
	expectBalance(t, other, otherWallet, 1_000_000)

	ownerWallet := createWallet(t, owner, "Cash", 10_000)
	status, r = payDebt(t, other, debt.ID, map[string]any{"amount": 1_000, "wallet_id": ownerWallet})
	expectStatus(t, status, http.StatusNotFound, r)
}

type goalResult struct {
	ID            string   `json:"id"`
	CurrentAmount float64  `json:"current_amount"`
	Remaining     float64  `json:"remaining"`
	ProgressPct   float64  `json:"progress_pct"`
	MonthsLeft    *int     `json:"months_left"`
	MonthlyNeeded *float64 `json:"monthly_needed"`
	Achieved      bool     `json:"achieved"`
}

func TestSavingGoalProgressAndAchievement(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 10_000_000)
	savings := createTypedWallet(t, u, "Tabungan Laptop", "savings", 2_000_000)

	deadline := todayWIB().AddDate(0, 4, 0).Format("2006-01-02") + "T00:00:00+07:00"
	status, r := call(t, http.MethodPost, "/api/goals", u.Token, map[string]any{
		"name": "Laptop", "target_amount": 10_000_000, "wallet_id": savings, "deadline": deadline,
	})
	expectStatus(t, status, http.StatusCreated, r)
	goal := decode[goalResult](t, r)
	if goal.CurrentAmount != 2_000_000 || goal.ProgressPct != 20 || goal.Remaining != 8_000_000 || goal.Achieved {
		t.Fatalf("progres awal salah: %+v", goal)
	}
	if goal.MonthsLeft == nil || *goal.MonthsLeft != 4 || goal.MonthlyNeeded == nil || *goal.MonthlyNeeded != 2_000_000 {
		t.Fatalf("estimasi per bulan salah: %+v", goal)
	}

	t.Run("satu dompet satu target", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/goals", u.Token, map[string]any{
			"name": "Lain", "target_amount": 1_000, "wallet_id": savings,
		})
		expectStatus(t, status, http.StatusConflict, r)
	})

	createTransfer(t, u, bca, savings, 7_000_000, 0)
	if got := countTitles(t, u, "Target Tabungan Tercapai"); got != 0 {
		t.Fatal("notifikasi target terkirim sebelum tercapai")
	}
	createTransfer(t, u, bca, savings, 1_000_000, 0)
	createTransfer(t, u, bca, savings, 500_000, 0)
	if got := countTitles(t, u, "Target Tabungan Tercapai"); got != 1 {
		t.Fatalf("notifikasi target tercapai %d, ingin tepat 1", got)
	}

	status, r = call(t, http.MethodGet, "/api/goals/"+goal.ID, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if g := decode[goalResult](t, r); !g.Achieved || g.ProgressPct != 100 || g.Remaining != 0 {
		t.Fatalf("status target setelah tercapai: %+v", g)
	}

	other := newUser(t)
	status, r = call(t, http.MethodGet, "/api/goals/"+goal.ID, other.Token, nil)
	expectStatus(t, status, http.StatusNotFound, r)
	status, r = call(t, http.MethodPost, "/api/goals", other.Token, map[string]any{
		"name": "Curi", "target_amount": 1_000, "wallet_id": bca,
	})
	expectStatus(t, status, http.StatusNotFound, r)
}

func TestDeletePayableDebtRestoresBalance(t *testing.T) {
	u := newUser(t)
	cash := createWallet(t, u, "Cash", 50_000)
	bca := createTypedWallet(t, u, "BCA", "bank", 1_000_000)

	debt := createDebt(t, u, map[string]any{
		"kind": "payable", "counterparty": "Siti", "amount": 500_000, "wallet_id": cash,
	})
	status, r := payDebt(t, u, debt.ID, map[string]any{"amount": 100_000, "wallet_id": bca})
	expectStatus(t, status, http.StatusCreated, r)
	expectBalance(t, u, cash, 550_000)
	expectBalance(t, u, bca, 900_000)

	status, r = call(t, http.MethodDelete, "/api/debts/"+debt.ID, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	expectBalance(t, u, cash, 50_000)
	expectBalance(t, u, bca, 1_000_000)
	expectReconciled(t, u)

	status, r = call(t, http.MethodGet, "/api/debts/"+debt.ID, u.Token, nil)
	expectStatus(t, status, http.StatusNotFound, r)
}
