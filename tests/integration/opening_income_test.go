package integration_test

import (
	"net/http"
	"testing"
)

func TestOpeningBalanceToIncome(t *testing.T) {
	u := newUser(t)
	bca := createTypedWallet(t, u, "BCA", "bank", 2_000_000)
	salary := createCategory(t, u, "Gaji", "income")
	food := createCategory(t, u, "Makan", "expense")
	createTransaction(t, u, bca, food, 300_000, "expense")

	t.Run("kategori pengeluaran ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/wallets/"+bca+"/opening-income", u.Token, map[string]any{"category_id": food})
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	status, r := call(t, http.MethodPost, "/api/wallets/"+bca+"/opening-income", u.Token, map[string]any{"category_id": salary})
	expectStatus(t, status, http.StatusOK, r)

	expectBalance(t, u, bca, 1_700_000)
	expectReconciled(t, u)
	if n := countRows(t, `SELECT COUNT(*) FROM transactions WHERE wallet_id = ? AND type = 'income' AND amount = 2000000 AND deleted_at IS NULL`, bca); n != 1 {
		t.Fatalf("%d transaksi pemasukan saldo awal, ingin 1", n)
	}
	if n := countRows(t, `SELECT COUNT(*) FROM wallets WHERE id = ? AND opening_balance = 0`, bca); n != 1 {
		t.Fatal("saldo awal belum menjadi 0")
	}

	h := walletHistory(t, u, bca, "?limit=100")
	for _, e := range h.Entries {
		if e.BalanceAfter < 0 {
			t.Fatalf("saldo berjalan minus %.0f; pemasukan saldo awal harus tercatat paling awal", e.BalanceAfter)
		}
	}

	t.Run("tidak bisa dua kali", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/wallets/"+bca+"/opening-income", u.Token, map[string]any{"category_id": salary})
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	t.Run("dompet baru tanpa transaksi", func(t *testing.T) {
		fresh := createTypedWallet(t, u, "Gaji baru", "bank", 4_000_000)
		status, r := call(t, http.MethodPost, "/api/wallets/"+fresh+"/opening-income", u.Token, map[string]any{"category_id": salary})
		expectStatus(t, status, http.StatusOK, r)
		expectBalance(t, u, fresh, 4_000_000)
		expectReconciled(t, u)
	})

	t.Run("dompet user lain", func(t *testing.T) {
		other := newUser(t)
		status, r := call(t, http.MethodPost, "/api/wallets/"+bca+"/opening-income", other.Token, map[string]any{"category_id": salary})
		expectStatus(t, status, http.StatusNotFound, r)
	})
}
