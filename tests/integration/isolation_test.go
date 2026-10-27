package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestDataIsolation memastikan setiap endpoint ber-ID menolak data milik pengguna
// lain, dan pengguna lain tidak bisa memakai dompet/kategori orang lain di body.
// Route baru yang memakai :id wajib didaftarkan di `owned` agar ikut diuji.
func TestDataIsolation(t *testing.T) {
	a := newUser(t)
	wallet := createTypedWallet(t, a, "BCA A", "bank", 5_000_000)
	cash := createWallet(t, a, "Tunai A", 100_000)
	savings := createTypedWallet(t, a, "Tabungan A", "savings", 0)
	food := createCategory(t, a, "Makan A", "expense")
	tx := createTransaction(t, a, wallet, food, 25_000, "expense")
	transfer := createTransfer(t, a, wallet, cash, 50_000, 0).ID
	status, r := call(t, http.MethodPost, "/api/budgets", a.Token, map[string]any{"category_id": food, "amount": 10_000, "month": 10, "year": 2026})
	expectStatus(t, status, http.StatusCreated, r)
	budget := decode[idResponse](t, r).ID
	rule := createRecurring(t, a, recurringBody(wallet, food, "expense", "monthly", 20_000, todayWIB().AddDate(0, 1, 0))).ID
	debt := createDebt(t, a, map[string]any{"kind": "payable", "counterparty": "Budi", "amount": 300_000, "wallet_id": wallet, "date": testDate})
	status, r = payDebt(t, a, debt.ID, map[string]any{"amount": 100_000, "wallet_id": wallet, "date": testDate})
	expectStatus(t, status, http.StatusCreated, r)
	payment := latestDebtMovement(t, a, debt.ID)
	status, r = call(t, http.MethodPost, "/api/goals", a.Token, map[string]any{"name": "Liburan", "target_amount": 1_000_000, "wallet_id": savings})
	expectStatus(t, status, http.StatusCreated, r)
	goal := decode[idResponse](t, r).ID
	notification := anyNotification(t, a)

	owned := map[string]string{
		"wallets": wallet, "categories": food, "transactions": tx, "transfers": transfer, "budgets": budget,
		"recurring": rule, "debts": debt.ID, "goals": goal, "notifications": notification,
	}
	bodies := map[string]map[string]any{
		"wallets":      {"name": "Diambil", "type": "cash", "actual_balance": 0, "date": testDate},
		"categories":   {"name": "Diambil", "type": "expense"},
		"transactions": txBody(wallet, food, 1, "expense"),
		"budgets":      {"amount": 1, "notes": "diambil"},
		"recurring":    recurringBody(wallet, food, "expense", "monthly", 1, todayWIB()),
		"debts":        {"counterparty": "Diambil", "amount": 1, "wallet_id": wallet, "date": testDate},
		"goals":        {"name": "Diambil", "target_amount": 1},
	}

	snapshot := func() map[string]string {
		out := map[string]string{}
		for res, id := range owned {
			if res == "notifications" {
				continue
			}
			status, r := call(t, http.MethodGet, "/api/"+res+"/"+id, a.Token, nil)
			expectStatus(t, status, http.StatusOK, r)
			out[res] = string(r.Data)
		}
		return out
	}
	before := snapshot()

	b := newUser(t)
	checked := 0
	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/") || strings.HasPrefix(route.Path, "/api/admin/") || strings.HasPrefix(route.Path, "/api/auth/") || !strings.Contains(route.Path, ":") {
			continue
		}
		resource := strings.Split(route.Path, "/")[2]
		id, ok := owned[resource]
		if !ok {
			t.Errorf("route %s %s belum dicakup tes isolasi", route.Method, route.Path)
			continue
		}
		path := strings.NewReplacer(":id", id, ":paymentId", payment).Replace(route.Path)
		status, r := call(t, route.Method, path, b.Token, bodies[resource])
		if status < 400 {
			t.Errorf("%s %s oleh pengguna lain berhasil (%d): %s", route.Method, route.Path, status, r.Data)
		}
		if route.Method == http.MethodGet && status != http.StatusNotFound && status != http.StatusForbidden {
			t.Errorf("%s %s oleh pengguna lain status %d, ingin 404/403", route.Method, route.Path, status)
		}
		checked++
	}
	if checked < 30 {
		t.Fatalf("hanya %d route ber-ID yang diuji", checked)
	}

	bWallet := createWallet(t, b, "Dompet B", 1_000_000)
	bFood := createCategory(t, b, "Makan B", "expense")
	bDebt := createDebt(t, b, map[string]any{"kind": "payable", "counterparty": "C", "amount": 50_000, "wallet_id": bWallet, "date": testDate})
	for name, req := range map[string]struct {
		path string
		body map[string]any
	}{
		"transaksi ke dompet A":      {"/api/transactions", txBody(wallet, bFood, 1_000, "expense")},
		"transaksi kategori A":       {"/api/transactions", txBody(bWallet, food, 1_000, "expense")},
		"pindahan ke dompet A":       {"/api/transfers", transferBody(bWallet, wallet, 1_000, 0)},
		"pindahan dari dompet A":     {"/api/transfers", transferBody(wallet, bWallet, 1_000, 0)},
		"anggaran tabungan dompet A": {"/api/budgets", map[string]any{"kind": "savings", "wallet_id": savings, "amount": 1_000, "month": 10, "year": 2026}},
		"anggaran tabungan target A": {"/api/budgets", map[string]any{"kind": "savings", "goal_id": goal, "amount": 1_000, "month": 10, "year": 2026}},
		"anggaran kategori A":        {"/api/budgets", map[string]any{"category_id": food, "amount": 1_000, "month": 10, "year": 2026}},
		"transaksi rutin dompet A":   {"/api/recurring", recurringBody(wallet, bFood, "expense", "monthly", 1_000, todayWIB())},
		"target memakai dompet A":    {"/api/goals", map[string]any{"name": "X", "target_amount": 1_000, "wallet_id": savings}},
		"hutang dicatat ke dompet A": {"/api/debts", map[string]any{"kind": "payable", "counterparty": "X", "amount": 1_000, "wallet_id": wallet, "date": testDate}},
		"bayar hutang dari dompet A": {"/api/debts/" + bDebt.ID + "/payments", map[string]any{"amount": 1_000, "wallet_id": wallet, "date": testDate}},
	} {
		if status, r := call(t, http.MethodPost, req.path, b.Token, req.body); status < 400 {
			t.Errorf("%s berhasil (%d): %s", name, status, r.Data)
		}
	}

	after := snapshot()
	for res, was := range before {
		if after[res] != was {
			t.Errorf("data %s milik A berubah oleh pengguna lain:\nsebelum %s\nsesudah %s", res, was, after[res])
		}
	}
	if got := balanceOf(t, a, wallet); got != 5_000_000-25_000-50_000+300_000-100_000 {
		t.Errorf("saldo dompet A berubah menjadi %.0f", got)
	}
}

func latestDebtMovement(t *testing.T, u testUser, debtID string) string {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/debts/"+debtID, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	d := decode[debtResult](t, r)
	for _, m := range d.Movements {
		if m.Kind != "initial" && m.Kind != "principal" {
			return m.ID
		}
	}
	return d.Movements[len(d.Movements)-1].ID
}

func anyNotification(t *testing.T, u testUser) string {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/notifications?page=1&limit=1", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"notifications"`
	}
	_ = json.Unmarshal(r.Data, &list)
	if len(list.Items) == 0 {
		t.Fatalf("pengguna A belum punya notifikasi untuk diuji: %s", r.Data)
	}
	return list.Items[0].ID
}
