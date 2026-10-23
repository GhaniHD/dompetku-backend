package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type notificationList struct {
	Notifications []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"notifications"`
	Total       int64 `json:"total"`
	TotalUnread int64 `json:"total_unread"`
	Page        int   `json:"page"`
	Limit       int   `json:"limit"`
}

func notifications(t *testing.T, u testUser, query string) notificationList {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/notifications"+query, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[notificationList](t, r)
}

func countTitles(t *testing.T, u testUser, prefix string) int {
	t.Helper()
	n := 0
	for _, item := range notifications(t, u, "?limit=100").Notifications {
		if strings.HasPrefix(item.Title, prefix) {
			n++
		}
	}
	return n
}

func createBudget(t *testing.T, u testUser, categoryID string, amount float64) {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/budgets", u.Token, map[string]any{
		"category_id": categoryID, "amount": amount, "month": 10, "year": 2026,
	})
	expectStatus(t, status, http.StatusCreated, r)
}

func TestBudgetAlertsOnlyWhenCrossingThreshold(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 1_000_000)
	food := createCategory(t, u, "Makan", "expense")
	createBudget(t, u, food, 100_000)

	steps := []struct {
		amount       float64
		wantWarning  int
		wantExceeded int
	}{
		{50_000, 0, 0},
		{35_000, 1, 0},
		{5_000, 1, 0},
		{20_000, 1, 1},
		{1_000, 1, 1},
	}
	for i, step := range steps {
		createTransaction(t, u, wallet, food, step.amount, "expense")
		if got := countTitles(t, u, "Anggaran Hampir Habis"); got != step.wantWarning {
			t.Fatalf("langkah %d: %d peringatan 80%%, ingin %d", i, got, step.wantWarning)
		}
		if got := countTitles(t, u, "Anggaran Terlampaui"); got != step.wantExceeded {
			t.Fatalf("langkah %d: %d peringatan terlampaui, ingin %d", i, got, step.wantExceeded)
		}
	}
}

func TestBudgetAlertWhenUpdateCrossesThreshold(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 1_000_000)
	food := createCategory(t, u, "Makan", "expense")
	createBudget(t, u, food, 100_000)
	tx := createTransaction(t, u, wallet, food, 10_000, "expense")

	status, r := call(t, http.MethodPut, "/api/transactions/"+tx, u.Token, map[string]any{"amount": 120_000})
	expectStatus(t, status, http.StatusOK, r)

	if got := countTitles(t, u, "Anggaran Terlampaui"); got != 1 {
		t.Fatalf("%d peringatan terlampaui setelah update, ingin 1", got)
	}
}

func TestGetBudgetsHasNoSideEffect(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 1_000_000)
	food := createCategory(t, u, "Makan", "expense")
	createBudget(t, u, food, 100_000)
	createTransaction(t, u, wallet, food, 95_000, "expense")

	before := notifications(t, u, "").Total
	for i := 0; i < 3; i++ {
		status, r := call(t, http.MethodGet, "/api/budgets?month=10&year=2026", u.Token, nil)
		expectStatus(t, status, http.StatusOK, r)
	}
	if after := notifications(t, u, "").Total; after != before {
		t.Fatalf("membuka daftar budget menambah %d notifikasi", after-before)
	}
}

func TestLowBalanceAlertOnlyWhenCrossing(t *testing.T) {
	u := newUser(t)
	cash := createWallet(t, u, "Cash", 100_000)
	bank := createTypedWallet(t, u, "BCA", "bank", 0)
	card := createTypedWallet(t, u, "Kartu Kredit", "credit_card", 0)
	food := createCategory(t, u, "Makan", "expense")
	salary := createCategory(t, u, "Gaji", "income")

	createTransaction(t, u, cash, food, 60_000, "expense")
	if got := countTitles(t, u, "Saldo Hampir Habis"); got != 1 {
		t.Fatalf("turun ke 40rb: %d notifikasi, ingin 1", got)
	}

	createTransaction(t, u, cash, food, 10_000, "expense")
	if got := countTitles(t, u, "Saldo Hampir Habis"); got != 1 {
		t.Fatalf("sudah di bawah ambang: %d notifikasi, ingin tetap 1", got)
	}

	createTransaction(t, u, cash, salary, 100_000, "income")
	createTransfer(t, u, cash, bank, 90_000, 0)
	if got := countTitles(t, u, "Saldo Hampir Habis"); got != 2 {
		t.Fatalf("transfer membuat saldo turun lagi: %d notifikasi, ingin 2", got)
	}

	createTransaction(t, u, card, food, 200_000, "expense")
	if got := countTitles(t, u, "Saldo Hampir Habis"); got != 2 {
		t.Fatal("kartu kredit tidak boleh memicu notifikasi saldo hampir habis")
	}
}

func TestNotificationPaginationAndOwnership(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "BCA", 0)
	salary := createCategory(t, u, "Gaji", "income")
	for i := 0; i < 5; i++ {
		createTransaction(t, u, wallet, salary, 1_000_000, "income")
	}

	page := notifications(t, u, "?page=2&limit=2")
	if page.Total != 5 || page.Page != 2 || page.Limit != 2 || len(page.Notifications) != 2 {
		t.Fatalf("pagination salah: total %d, page %d, limit %d, isi %d", page.Total, page.Page, page.Limit, len(page.Notifications))
	}

	status, r := call(t, http.MethodGet, "/api/notifications?limit=1000", u.Token, nil)
	expectStatus(t, status, http.StatusBadRequest, r)

	target := page.Notifications[0].ID
	other := newUser(t)
	for _, req := range []struct{ method, path string }{
		{http.MethodPatch, fmt.Sprintf("/api/notifications/%s/read", target)},
		{http.MethodDelete, "/api/notifications/" + target},
	} {
		status, r := call(t, req.method, req.path, other.Token, nil)
		expectStatus(t, status, http.StatusNotFound, r)
	}

	status, r = call(t, http.MethodPatch, fmt.Sprintf("/api/notifications/%s/read", target), u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if unread := notifications(t, u, "").TotalUnread; unread != 4 {
		t.Fatalf("notifikasi belum dibaca %d, ingin 4", unread)
	}
}
