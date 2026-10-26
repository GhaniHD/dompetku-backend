package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

type budgetPlan struct {
	Savings []struct {
		Name      string  `json:"name"`
		Planned   float64 `json:"planned"`
		Deposited float64 `json:"deposited"`
		Remain    float64 `json:"remain"`
	} `json:"savings"`
	SavingsPlanned   float64 `json:"savings_planned"`
	SavingsDeposited float64 `json:"savings_deposited"`
	FreeMoney        *struct {
		Spendable   float64 `json:"spendable"`
		BudgetLeft  float64 `json:"budget_left"`
		SavingsLeft float64 `json:"savings_left"`
		Amount      float64 `json:"amount"`
	} `json:"free_money"`
}

func TestSavingsInBudget(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 5_000_000)
	phone := createTypedWallet(t, u, "Tabungan HP", "savings", 1_000_000)
	trip := createTypedWallet(t, u, "Tabungan Liburan", "savings", 0)
	food := createCategory(t, u, "Makan", "expense")

	now := todayWIB()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	deadline := monthStart.AddDate(0, 3, 0).Format("2006-01-02") + "T00:00:00+07:00"
	status, r := call(t, http.MethodPost, "/api/goals", u.Token, map[string]any{
		"name": "HP baru", "target_amount": 4_000_000, "wallet_id": phone, "deadline": deadline,
	})
	expectStatus(t, status, http.StatusCreated, r)
	status, r = call(t, http.MethodPost, "/api/goals", u.Token, map[string]any{
		"name": "Liburan", "target_amount": 3_000_000, "wallet_id": trip,
	})
	expectStatus(t, status, http.StatusCreated, r)

	createBudgetFor(t, u, food, 1_000_000, int(now.Month()), now.Year())
	day := now.Format("2006-01-02")
	expenseOn(t, u, bca, food, 250_000, day)
	transferOn(t, u, bca, phone, 400_000, 0, day)
	transferOn(t, u, bca, trip, 100_000, 0, day)

	status, r = call(t, http.MethodGet, "/api/budgets/current", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	plan := decode[budgetPlan](t, r)

	if len(plan.Savings) != 2 {
		t.Fatalf("%d baris tabungan, ingin 2: %+v", len(plan.Savings), plan.Savings)
	}
	rows := map[string]int{}
	for i, s := range plan.Savings {
		rows[s.Name] = i
	}
	hp := plan.Savings[rows["HP baru"]]
	if hp.Planned != 1_000_000 || hp.Deposited != 400_000 || hp.Remain != 600_000 {
		t.Fatalf("HP baru %+v, ingin rencana 1jt, setor 400rb, sisa 600rb", hp)
	}
	trip2 := plan.Savings[rows["Liburan"]]
	if trip2.Planned != 0 || trip2.Deposited != 100_000 || trip2.Remain != 0 {
		t.Fatalf("Liburan tanpa tenggat %+v", trip2)
	}
	if plan.SavingsPlanned != 1_000_000 || plan.SavingsDeposited != 500_000 {
		t.Fatalf("total tabungan %+v", plan)
	}

	f := plan.FreeMoney
	if f == nil {
		t.Fatal("free_money kosong untuk periode berjalan")
	}
	// BCA 5jt − 250rb − 400rb − 100rb; dompet tabungan tidak dihitung.
	if f.Spendable != 4_250_000 || f.BudgetLeft != 750_000 || f.SavingsLeft != 600_000 || f.Amount != 2_900_000 {
		t.Fatalf("uang bebas %+v", *f)
	}

	prev := monthStart.AddDate(0, -1, 0)
	status, r = call(t, http.MethodGet, fmt.Sprintf("/api/budgets?month=%d&year=%d", prev.Month(), prev.Year()), u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if past := decode[budgetPlan](t, r); past.FreeMoney != nil || len(past.Savings) != 0 {
		t.Fatalf("periode lalu %+v, ingin tanpa uang bebas dan tanpa target yang belum dibuat", past)
	}
}
