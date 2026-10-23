package integration_test

import (
	"net/http"
	"testing"
)

func TestCreateTransactionUpdatesBalance(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 100000)
	food := createCategory(t, u, "Makan", "expense")
	salary := createCategory(t, u, "Gaji", "income")

	createTransaction(t, u, wallet, food, 20000, "expense")
	expectBalance(t, u, wallet, 80000)

	createTransaction(t, u, wallet, salary, 150_000_000, "income")
	expectBalance(t, u, wallet, 150_080_000)

	expectReconciled(t, u)
}

func TestCreateTransactionValidation(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 100000)
	food := createCategory(t, u, "Makan", "expense")

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"amount negatif", txBody(wallet, food, -5000, "expense"), http.StatusBadRequest},
		{"amount nol", txBody(wallet, food, 0, "expense"), http.StatusBadRequest},
		{"tipe tidak dikenal", txBody(wallet, food, 5000, "transfer"), http.StatusBadRequest},
		{"income dengan kategori expense", txBody(wallet, food, 5000, "income"), http.StatusBadRequest},
		{"wallet tidak ada", txBody("00000000-0000-0000-0000-000000000001", food, 5000, "expense"), http.StatusNotFound},
		{"kategori tidak ada", txBody(wallet, "00000000-0000-0000-0000-000000000001", 5000, "expense"), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, r := call(t, http.MethodPost, "/api/transactions", u.Token, tc.body)
			expectStatus(t, status, tc.want, r)
		})
	}

	expectBalance(t, u, wallet, 100000)
	if n := countRows(t, "SELECT COUNT(*) FROM transactions WHERE user_id = ?", u.ID); n != 0 {
		t.Fatalf("%d transaksi tersimpan padahal semua request ditolak", n)
	}
}

func TestUpdateTransactionMovesWallet(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 100000)
	cash := createWallet(t, u, "Cash", 0)
	food := createCategory(t, u, "Makan", "expense")
	snack := createCategory(t, u, "Jajan", "expense")
	tx := createTransaction(t, u, gopay, food, 20000, "expense")

	status, r := call(t, http.MethodPut, "/api/transactions/"+tx, u.Token, map[string]any{
		"wallet_id": cash, "category_id": snack,
	})
	expectStatus(t, status, http.StatusOK, r)

	got := decode[struct {
		Wallet   struct{ Name string } `json:"wallet"`
		Category struct{ Name string } `json:"category"`
	}](t, r)
	if got.Wallet.Name != "Cash" || got.Category.Name != "Jajan" {
		t.Fatalf("perubahan tidak tersimpan: wallet %q, kategori %q", got.Wallet.Name, got.Category.Name)
	}

	expectBalance(t, u, gopay, 100000)
	expectBalance(t, u, cash, -20000)
	expectReconciled(t, u)
}

func TestUpdateTransactionPartialFields(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 100000)
	food := createCategory(t, u, "Makan", "expense")
	salary := createCategory(t, u, "Gaji", "income")

	status, r := call(t, http.MethodPost, "/api/transactions", u.Token, map[string]any{
		"wallet_id": wallet, "category_id": food, "amount": 20000, "type": "expense",
		"note": "nasi padang", "date": testDate,
	})
	expectStatus(t, status, http.StatusCreated, r)
	tx := decode[idResponse](t, r).ID

	t.Run("note bisa dikosongkan", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/transactions/"+tx, u.Token, map[string]any{"note": ""})
		expectStatus(t, status, http.StatusOK, r)
		if note := decode[struct{ Note string }](t, r).Note; note != "" {
			t.Fatalf("note masih %q", note)
		}
	})

	t.Run("amount nol ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/transactions/"+tx, u.Token, map[string]any{"amount": 0})
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	t.Run("ganti tipe tanpa ganti kategori ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/transactions/"+tx, u.Token, map[string]any{"type": "income"})
		expectStatus(t, status, http.StatusBadRequest, r)
		expectBalance(t, u, wallet, 80000)
	})

	t.Run("ganti tipe dan kategori sekaligus", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/transactions/"+tx, u.Token, map[string]any{
			"type": "income", "category_id": salary, "amount": 50000,
		})
		expectStatus(t, status, http.StatusOK, r)
		expectBalance(t, u, wallet, 150000)
	})

	expectReconciled(t, u)
}

func TestDeleteTransactionRestoresBalance(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 100000)
	food := createCategory(t, u, "Makan", "expense")
	tx := createTransaction(t, u, wallet, food, 30000, "expense")

	status, r := call(t, http.MethodDelete, "/api/transactions/"+tx, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	expectBalance(t, u, wallet, 100000)

	status, r = call(t, http.MethodGet, "/api/transactions/"+tx, u.Token, nil)
	expectStatus(t, status, http.StatusNotFound, r)
	expectReconciled(t, u)
}

func TestTransactionOwnership(t *testing.T) {
	owner := newUser(t)
	ownerWallet := createWallet(t, owner, "Gopay", 100000)
	ownerFood := createCategory(t, owner, "Makan", "expense")
	ownerTx := createTransaction(t, owner, ownerWallet, ownerFood, 10000, "expense")

	other := newUser(t)
	otherWallet := createWallet(t, other, "Dompet Lain", 0)
	otherFood := createCategory(t, other, "Jajan", "expense")

	t.Run("pakai wallet user lain", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/transactions", other.Token, txBody(ownerWallet, otherFood, 1000, "expense"))
		expectStatus(t, status, http.StatusNotFound, r)
	})
	t.Run("pakai kategori user lain", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/transactions", other.Token, txBody(otherWallet, ownerFood, 1000, "expense"))
		expectStatus(t, status, http.StatusNotFound, r)
	})
	t.Run("pindahkan transaksi ke wallet user lain", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/transactions/"+ownerTx, owner.Token, map[string]any{"wallet_id": otherWallet})
		expectStatus(t, status, http.StatusNotFound, r)
	})
	t.Run("baca, ubah, hapus transaksi user lain", func(t *testing.T) {
		for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			status, r := call(t, m, "/api/transactions/"+ownerTx, other.Token, map[string]any{"note": "x"})
			expectStatus(t, status, http.StatusNotFound, r)
		}
	})

	if n := countRows(t, "SELECT COUNT(*) FROM transactions WHERE user_id = ?", other.ID); n != 0 {
		t.Fatalf("%d transaksi yatim tersimpan untuk user lain", n)
	}
	expectBalance(t, owner, ownerWallet, 90000)
	expectBalance(t, other, otherWallet, 0)
}

func TestTransactionFilters(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 100000)
	cash := createWallet(t, u, "Cash", 100000)
	food := createCategory(t, u, "Makan", "expense")
	salary := createCategory(t, u, "Gaji", "income")
	createTransaction(t, u, gopay, food, 1000, "expense")
	createTransaction(t, u, cash, food, 2000, "expense")
	createTransaction(t, u, cash, salary, 3000, "income")

	cases := []struct {
		query string
		want  int
	}{
		{"", 3},
		{"?wallet_id=" + cash, 2},
		{"?category_id=" + food, 2},
		{"?wallet_id=" + cash + "&type=income", 1},
		{"?start_date=2026-10-01T00:00:00Z&end_date=2026-10-31T00:00:00Z", 3},
		{"?start_date=2026-11-01T00:00:00Z", 0},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			status, r := call(t, http.MethodGet, "/api/transactions"+tc.query, u.Token, nil)
			expectStatus(t, status, http.StatusOK, r)
			if got := len(decode[[]idResponse](t, r)); got != tc.want {
				t.Fatalf("dapat %d transaksi, ingin %d", got, tc.want)
			}
		})
	}

	status, r := call(t, http.MethodGet, "/api/transactions?wallet_id=bukan-uuid", u.Token, nil)
	expectStatus(t, status, http.StatusBadRequest, r)
}

func TestConcurrentTransactionsKeepExactBalance(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Gopay", 100000)
	food := createCategory(t, u, "Makan", "expense")

	const n = 40
	statuses := callConcurrently(t, n, func(int) (string, string, string, any) {
		return http.MethodPost, "/api/transactions", u.Token, txBody(wallet, food, 1000, "expense")
	})
	expectAllStatus(t, statuses, http.StatusCreated)
	expectBalance(t, u, wallet, 100000-n*1000)
	expectReconciled(t, u)
}
