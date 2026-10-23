package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

type budgetSummary struct {
	Month       int    `json:"month"`
	Year        int    `json:"year"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	StartDay    int    `json:"budget_start_day"`
	Budgets     []struct {
		Spent       float64 `json:"spent"`
		PeriodStart string  `json:"period_start"`
		PeriodEnd   string  `json:"period_end"`
	} `json:"budgets"`
}

func setBudgetStartDay(t *testing.T, u testUser, day int) {
	t.Helper()
	status, r := call(t, http.MethodPut, "/api/profile/budget-cycle", u.Token, map[string]any{"start_day": day})
	expectStatus(t, status, http.StatusOK, r)
	if got := decode[struct {
		StartDay int `json:"budget_start_day"`
	}](t, r).StartDay; got != day {
		t.Fatalf("budget_start_day %d, ingin %d", got, day)
	}
}

func createBudgetFor(t *testing.T, u testUser, categoryID string, amount float64, month, year int) {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/budgets", u.Token, map[string]any{
		"category_id": categoryID, "amount": amount, "month": month, "year": year,
	})
	expectStatus(t, status, http.StatusCreated, r)
}

func expenseOn(t *testing.T, u testUser, walletID, categoryID string, amount float64, date string) {
	t.Helper()
	body := txBody(walletID, categoryID, amount, "expense")
	body["date"] = date + "T09:00:00+07:00"
	status, r := call(t, http.MethodPost, "/api/transactions", u.Token, body)
	expectStatus(t, status, http.StatusCreated, r)
}

func TestDefaultBudgetCycleIsCalendarMonth(t *testing.T) {
	u := newUser(t)

	status, r := call(t, http.MethodGet, "/api/profile", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if day := decode[struct {
		StartDay int `json:"budget_start_day"`
	}](t, r).StartDay; day != 1 {
		t.Fatalf("budget_start_day default %d, ingin 1", day)
	}

	food := createCategory(t, u, "Makan", "expense")
	createBudgetFor(t, u, food, 1_000_000, 10, 2026)
	status, r = call(t, http.MethodGet, "/api/budgets?month=10&year=2026", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	summary := decode[budgetSummary](t, r)
	if summary.PeriodStart != "2026-10-01" || summary.PeriodEnd != "2026-10-31" {
		t.Fatalf("periode default %s – %s, ingin 2026-10-01 – 2026-10-31", summary.PeriodStart, summary.PeriodEnd)
	}
}

func TestBudgetCycleFollowsPayday(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "BCA", 10_000_000)
	food := createCategory(t, u, "Makan", "expense")
	setBudgetStartDay(t, u, 25)
	createBudgetFor(t, u, food, 1_000_000, 10, 2026)

	expenseOn(t, u, wallet, food, 100_000, "2026-10-24")
	expenseOn(t, u, wallet, food, 200_000, "2026-10-25")
	expenseOn(t, u, wallet, food, 300_000, "2026-11-24")
	expenseOn(t, u, wallet, food, 400_000, "2026-11-25")

	status, r := call(t, http.MethodGet, "/api/budgets?month=10&year=2026", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	summary := decode[budgetSummary](t, r)

	if summary.StartDay != 25 || summary.PeriodStart != "2026-10-25" || summary.PeriodEnd != "2026-11-24" {
		t.Fatalf("periode summary salah: hari %d, %s – %s", summary.StartDay, summary.PeriodStart, summary.PeriodEnd)
	}
	if len(summary.Budgets) != 1 {
		t.Fatalf("jumlah budget %d, ingin 1", len(summary.Budgets))
	}
	b := summary.Budgets[0]
	if b.Spent != 500_000 {
		t.Fatalf("terpakai %.0f, ingin 500000 (hanya 25 Okt – 24 Nov)", b.Spent)
	}
	if b.PeriodStart != "2026-10-25" || b.PeriodEnd != "2026-11-24" {
		t.Fatalf("periode budget %s – %s", b.PeriodStart, b.PeriodEnd)
	}
}

func TestBudgetAlertFollowsPayday(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "BCA", 10_000_000)
	food := createCategory(t, u, "Makan", "expense")
	setBudgetStartDay(t, u, 25)
	createBudgetFor(t, u, food, 100_000, 10, 2026)

	expenseOn(t, u, wallet, food, 85_000, "2026-11-10")

	if got := countTitles(t, u, "Anggaran Hampir Habis"); got != 1 {
		t.Fatalf("transaksi 10 Nov masuk periode budget Oktober (gajian 25), ingin 1 peringatan, dapat %d", got)
	}
}

func TestBudgetCycleValidation(t *testing.T) {
	u := newUser(t)
	for _, day := range []int{0, 32, -1} {
		t.Run(fmt.Sprint(day), func(t *testing.T) {
			status, r := call(t, http.MethodPut, "/api/profile/budget-cycle", u.Token, map[string]any{"start_day": day})
			expectStatus(t, status, http.StatusBadRequest, r)
		})
	}
}

func TestCurrentBudgetsUsesPayday(t *testing.T) {
	u := newUser(t)
	food := createCategory(t, u, "Makan", "expense")
	// Tanggal 31 = hari terakhir tiap bulan, sehingga periode berjalan berbeda dari
	// bulan kalender di hampir semua hari dan test ini benar-benar menguji siklus.
	setBudgetStartDay(t, u, 31)

	now := time.Now().In(time.FixedZone("WIB", 7*60*60))
	period := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastDay := period.AddDate(0, 1, -1).Day()
	if now.Day() < lastDay {
		period = period.AddDate(0, -1, 0)
	}
	createBudgetFor(t, u, food, 500_000, int(period.Month()), period.Year())

	status, r := call(t, http.MethodGet, "/api/budgets/current", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	summary := decode[budgetSummary](t, r)

	if summary.Month != int(period.Month()) || summary.Year != period.Year() {
		t.Fatalf("periode berjalan %d/%d, ingin %d/%d", summary.Month, summary.Year, int(period.Month()), period.Year())
	}
	if len(summary.Budgets) != 1 {
		t.Fatalf("budget periode berjalan %d, ingin 1", len(summary.Budgets))
	}
}
