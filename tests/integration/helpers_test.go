package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
)

const testDate = "2026-10-21T09:00:00+07:00"

type apiResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Token   string          `json:"token"`
}

type idResponse struct {
	ID string `json:"id"`
}

// tryCall tidak memanggil t.Fatal sehingga aman dipakai dari goroutine.
func tryCall(method, path, token string, body any) (int, apiResponse, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, apiResponse{}, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		return 0, apiResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, apiResponse{}, err
	}
	defer res.Body.Close()

	var out apiResponse
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out, nil
}

func call(t *testing.T, method, path, token string, body any) (int, apiResponse) {
	t.Helper()
	status, out, err := tryCall(method, path, token, body)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return status, out
}

// callConcurrently menjalankan n request sekaligus dan mengembalikan status masing-masing.
func callConcurrently(t *testing.T, n int, request func(i int) (method, path, token string, body any)) []int {
	t.Helper()
	statuses := make([]int, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			method, path, token, body := request(i)
			statuses[i], _, errs[i] = tryCall(method, path, token, body)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request paralel %d: %v", i, err)
		}
	}
	return statuses
}

func expectAllStatus(t *testing.T, statuses []int, want int) {
	t.Helper()
	for i, s := range statuses {
		if s != want {
			t.Fatalf("request %d berstatus %d, ingin %d", i, s, want)
		}
	}
}

func decode[T any](t *testing.T, r apiResponse) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.Data, &v); err != nil {
		t.Fatalf("decode data: %v (raw: %s)", err, r.Data)
	}
	return v
}

func expectStatus(t *testing.T, got, want int, r apiResponse) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d, ingin %d (message: %q)", got, want, r.Message)
	}
}

type testUser struct {
	ID    string
	Email string
	Token string
}

func newUser(t *testing.T) testUser {
	t.Helper()
	u := registerPending(t, "Tester")
	approveDirect(t, u.ID)
	u.Token = login(t, u.Email, "rahasia123")
	return u
}

// registerPending mendaftar tanpa undangan; akun menunggu persetujuan admin.
func registerPending(t *testing.T, name string) testUser {
	t.Helper()
	email := fmt.Sprintf("user-%s@test.dompetku", uuid.NewString()[:8])
	status, r := call(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name": name, "email": email, "password": "rahasia123",
	})
	expectStatus(t, status, http.StatusCreated, r)
	data := decode[struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		User   struct {
			ID string `json:"id"`
		} `json:"user"`
	}](t, r)
	if data.Status != "pending" || data.Token != "" {
		t.Fatalf("pendaftaran tanpa undangan harus pending tanpa token, dapat status %q", data.Status)
	}
	return testUser{ID: data.User.ID, Email: email}
}

func approveDirect(t *testing.T, userID string) {
	t.Helper()
	if err := gdb.Exec(`UPDATE users SET status = 'active' WHERE id = ?`, userID).Error; err != nil {
		t.Fatalf("approve: %v", err)
	}
}

func login(t *testing.T, email, password string) string {
	t.Helper()
	status, out, err := tryCall(http.MethodPost, "/api/auth/login", "", map[string]any{"email": email, "password": password})
	if err != nil || status != http.StatusOK || out.Token == "" {
		t.Fatalf("login %s: status %d (%q)", email, status, out.Message)
	}
	return out.Token
}

func createWallet(t *testing.T, u testUser, name string, balance float64) string {
	t.Helper()
	return createTypedWallet(t, u, name, "cash", balance)
}

func createTypedWallet(t *testing.T, u testUser, name, kind string, balance float64) string {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/wallets", u.Token, map[string]any{"name": name, "type": kind, "balance": balance})
	expectStatus(t, status, http.StatusCreated, r)
	return decode[idResponse](t, r).ID
}

func createCategory(t *testing.T, u testUser, name, kind string) string {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/categories", u.Token, map[string]any{"name": name, "type": kind})
	expectStatus(t, status, http.StatusCreated, r)
	return decode[idResponse](t, r).ID
}

func txBody(walletID, categoryID string, amount float64, kind string) map[string]any {
	return map[string]any{
		"wallet_id": walletID, "category_id": categoryID,
		"amount": amount, "type": kind, "date": testDate,
	}
}

func createTransaction(t *testing.T, u testUser, walletID, categoryID string, amount float64, kind string) string {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/transactions", u.Token, txBody(walletID, categoryID, amount, kind))
	expectStatus(t, status, http.StatusCreated, r)
	return decode[idResponse](t, r).ID
}

func balanceOf(t *testing.T, u testUser, walletID string) float64 {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/wallets/"+walletID, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[struct {
		Balance float64 `json:"balance"`
	}](t, r).Balance
}

func expectBalance(t *testing.T, u testUser, walletID string, want float64) {
	t.Helper()
	if got := balanceOf(t, u, walletID); got != want {
		t.Fatalf("saldo %.2f, ingin %.2f", got, want)
	}
}

func countRows(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := gdb.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	return n
}

// expectReconciled memastikan invarian utama untuk semua dompet milik user:
// balance = opening_balance + Σ transaksi + Σ transfer masuk − Σ transfer keluar + Σ penyesuaian.
func expectReconciled(t *testing.T, u testUser) {
	t.Helper()
	n := countRows(t, `
		SELECT COUNT(*) FROM wallets w
		WHERE w.user_id = ? AND w.balance <> w.opening_balance
			+ COALESCE((SELECT SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END)
			            FROM transactions t WHERE t.wallet_id = w.id AND t.deleted_at IS NULL), 0)
			+ COALESCE((SELECT SUM(x.amount) FROM transfers x WHERE x.to_wallet_id = w.id AND x.deleted_at IS NULL), 0)
			- COALESCE((SELECT SUM(x.amount) FROM transfers x WHERE x.from_wallet_id = w.id AND x.deleted_at IS NULL), 0)
			+ COALESCE((SELECT SUM(a.amount) FROM balance_adjustments a WHERE a.wallet_id = w.id AND a.deleted_at IS NULL), 0)
			+ COALESCE((SELECT SUM(m.wallet_delta) FROM debt_movements m WHERE m.wallet_id = w.id AND m.deleted_at IS NULL), 0)`,
		u.ID)
	if n != 0 {
		t.Fatalf("%d dompet saldonya tidak cocok dengan riwayat transaksi/transfer", n)
	}
}
