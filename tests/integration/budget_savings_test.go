package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

type budgetLine struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Amount   float64 `json:"amount"`
	Spent    float64 `json:"spent"`
	Remain   float64 `json:"remain"`
	Status   string  `json:"status"`
	Category struct {
		Name string `json:"name"`
	} `json:"category"`
	Wallet *struct {
		Name string `json:"name"`
	} `json:"wallet"`
	Goal *struct {
		Name string `json:"name"`
	} `json:"goal"`
}

type budgetPlan struct {
	TotalBudget      float64      `json:"total_budget"`
	TotalSpent       float64      `json:"total_spent"`
	Budgets          []budgetLine `json:"budgets"`
	SavingsPlanned   float64      `json:"savings_planned"`
	SavingsDeposited float64      `json:"savings_deposited"`
	FreeMoney        *struct {
		Spendable   float64 `json:"spendable"`
		BudgetLeft  float64 `json:"budget_left"`
		SavingsLeft float64 `json:"savings_left"`
		Amount      float64 `json:"amount"`
	} `json:"free_money"`
}

func budgetsFor(t *testing.T, u testUser, month time.Month, year int) budgetPlan {
	t.Helper()
	status, r := call(t, http.MethodGet, fmt.Sprintf("/api/budgets?month=%d&year=%d", month, year), u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[budgetPlan](t, r)
}

func (p budgetPlan) line(t *testing.T, name string) budgetLine {
	t.Helper()
	for _, b := range p.Budgets {
		switch {
		case b.Goal != nil && b.Goal.Name == name, b.Wallet != nil && b.Goal == nil && b.Wallet.Name == name, b.Category.Name == name:
			return b
		}
	}
	t.Fatalf("baris anggaran %q tidak ada: %+v", name, p.Budgets)
	return budgetLine{}
}

func TestSavingsBudget(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 5_000_000)
	phone := createTypedWallet(t, u, "Tabungan HP", "savings", 1_000_000)
	cold := createTypedWallet(t, u, "Tabungan Dingin", "savings", 500_000)
	card := createTypedWallet(t, u, "Kartu Kredit", "credit_card", 0)
	food := createCategory(t, u, "Makan", "expense")
	transport := createCategory(t, u, "Transportasi", "expense")
	needs := createCategory(t, u, "Keperluan", "expense")

	status, r := call(t, http.MethodPost, "/api/goals", u.Token, map[string]any{"name": "HP baru", "target_amount": 4_000_000, "wallet_id": phone})
	expectStatus(t, status, http.StatusCreated, r)
	goal := decode[idResponse](t, r).ID

	now := todayWIB()
	month, year := int(now.Month()), now.Year()
	day := now.Format("2006-01-02")
	createBudgetFor(t, u, food, 1_000_000, month, year)
	createBudgetFor(t, u, transport, 100_000, month, year)

	newSavings := func(body map[string]any) (int, apiResponse) {
		body["kind"], body["month"], body["year"] = "savings", month, year
		return call(t, http.MethodPost, "/api/budgets", u.Token, body)
	}
	status, r = newSavings(map[string]any{"goal_id": goal, "amount": 1_000_000})
	expectStatus(t, status, http.StatusCreated, r)
	status, r = newSavings(map[string]any{"wallet_id": cold, "amount": 300_000})
	expectStatus(t, status, http.StatusCreated, r)

	for name, body := range map[string]map[string]any{
		"dompet yang sama dua kali": {"wallet_id": phone, "amount": 1},
		"kartu kredit":              {"wallet_id": card, "amount": 1},
		"tanpa tujuan":              {"amount": 1},
	} {
		if status, r := newSavings(body); status != http.StatusBadRequest {
			t.Errorf("%s: status %d (%s), ingin 400", name, status, r.Message)
		}
	}

	expenseOn(t, u, bca, food, 250_000, day)
	expenseOn(t, u, bca, transport, 100_000, day)
	transferOn(t, u, bca, phone, 400_000, 0, day)
	expenseOn(t, u, cold, needs, 135_000, day)

	plan := budgetsFor(t, u, now.Month(), year)
	if plan.TotalBudget != 1_100_000 || plan.TotalSpent != 350_000 {
		t.Fatalf("total anggaran pengeluaran %v/%v, ingin 1,1jt/350rb (tabungan tidak ikut)", plan.TotalSpent, plan.TotalBudget)
	}
	if got := plan.line(t, "Transportasi").Status; got != "used_up" {
		t.Errorf("anggaran pas habis berstatus %q, ingin used_up", got)
	}
	hp := plan.line(t, "HP baru")
	if hp.Kind != "savings" || hp.Wallet == nil || hp.Wallet.Name != "Tabungan HP" || hp.Spent != 400_000 || hp.Remain != 600_000 || hp.Status != "in_progress" {
		t.Fatalf("tabungan HP %+v", hp)
	}
	dingin := plan.line(t, "Tabungan Dingin")
	if dingin.Spent != -135_000 || dingin.Remain != 435_000 {
		t.Fatalf("tabungan dingin setelah diambil 135rb: %+v, ingin setor -135rb sisa 435rb", dingin)
	}
	if plan.SavingsPlanned != 1_300_000 || plan.SavingsDeposited != 265_000 {
		t.Fatalf("total tabungan rencana %v disetor %v", plan.SavingsPlanned, plan.SavingsDeposited)
	}

	f := plan.FreeMoney
	if f == nil {
		t.Fatal("free_money kosong untuk periode berjalan")
	}
	// BCA 5jt − 250rb − 100rb − 400rb; dompet tabungan tidak dihitung siap pakai.
	if f.Spendable != 4_250_000 || f.BudgetLeft != 750_000 || f.SavingsLeft != 1_035_000 || f.Amount != 2_465_000 {
		t.Fatalf("uang bebas %+v", *f)
	}

	transferOn(t, u, bca, phone, 700_000, 0, day)
	if hp := budgetsFor(t, u, now.Month(), year).line(t, "HP baru"); hp.Status != "achieved" || hp.Remain != 0 {
		t.Fatalf("setelah setor 1,1jt dari rencana 1jt: %+v", hp)
	}

	next := time.Date(year, now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	status, r = call(t, http.MethodPost, "/api/budgets/copy", u.Token, map[string]any{
		"from_month": month, "from_year": year, "to_month": int(next.Month()), "to_year": next.Year(),
	})
	expectStatus(t, status, http.StatusCreated, r)
	copied := budgetsFor(t, u, next.Month(), next.Year())
	if len(copied.Budgets) != 4 || copied.SavingsPlanned != 1_300_000 || copied.FreeMoney != nil {
		t.Fatalf("salinan periode berikut %+v, ingin 4 baris termasuk 2 tabungan dan tanpa uang bebas", copied)
	}
}

func TestGoalAchievementFollowsBalance(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 1_000_000)
	small := createTypedWallet(t, u, "Tabungan Kecil", "savings", 0)
	needs := createCategory(t, u, "Keperluan", "expense")
	status, r := call(t, http.MethodPost, "/api/goals", u.Token, map[string]any{"name": "Kecil", "target_amount": 100_000, "wallet_id": small})
	expectStatus(t, status, http.StatusCreated, r)
	goal := decode[idResponse](t, r).ID

	achieved := func() bool {
		status, r := call(t, http.MethodGet, "/api/goals/"+goal, u.Token, nil)
		expectStatus(t, status, http.StatusOK, r)
		return decode[struct {
			Achieved bool `json:"achieved"`
		}](t, r).Achieved
	}

	createTransfer(t, u, bca, small, 100_000, 0)
	if !achieved() {
		t.Fatal("target belum tercapai setelah saldo = target")
	}
	createTransaction(t, u, small, needs, 30_000, "expense")
	if achieved() {
		t.Fatal("target masih tercapai setelah uangnya diambil")
	}
	createTransfer(t, u, bca, small, 30_000, 0)
	if !achieved() {
		t.Fatal("target tidak tercapai lagi setelah saldo kembali penuh")
	}
	if got := countTitles(t, u, "Target Tabungan Tercapai"); got != 2 {
		t.Fatalf("notifikasi tercapai %d, ingin 2 (sekali per pencapaian)", got)
	}
}
