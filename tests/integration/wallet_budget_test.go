package integration_test

import (
	"net/http"
	"testing"
)

func TestManualBalanceEditKeepsReconciliation(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "BCA", 5_000_000)
	food := createCategory(t, u, "Makan", "expense")
	createTransaction(t, u, wallet, food, 425_000, "expense")

	status, r := call(t, http.MethodPut, "/api/wallets/"+wallet, u.Token, map[string]any{"balance": 4_500_000})
	expectStatus(t, status, http.StatusOK, r)
	expectBalance(t, u, wallet, 4_500_000)
	expectReconciled(t, u)

	createTransaction(t, u, wallet, food, 100_000, "expense")
	expectBalance(t, u, wallet, 4_400_000)
	expectReconciled(t, u)
}

func TestWalletOwnership(t *testing.T) {
	owner := newUser(t)
	wallet := createWallet(t, owner, "Gopay", 100000)
	other := newUser(t)

	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		status, r := call(t, m, "/api/wallets/"+wallet, other.Token, map[string]any{"name": "dicuri", "balance": 0})
		expectStatus(t, status, http.StatusNotFound, r)
	}
	expectBalance(t, owner, wallet, 100000)
}

func TestBudgetCategoryRules(t *testing.T) {
	owner := newUser(t)
	food := createCategory(t, owner, "Makan", "expense")
	salary := createCategory(t, owner, "Gaji", "income")
	other := newUser(t)

	budget := func(token, category string) (int, apiResponse) {
		return call(t, http.MethodPost, "/api/budgets", token, map[string]any{
			"category_id": category, "amount": 1_000_000, "month": 10, "year": 2026,
		})
	}

	t.Run("kategori user lain ditolak", func(t *testing.T) {
		status, r := budget(other.Token, food)
		expectStatus(t, status, http.StatusBadRequest, r)
	})
	t.Run("kategori income ditolak", func(t *testing.T) {
		status, r := budget(owner.Token, salary)
		expectStatus(t, status, http.StatusBadRequest, r)
	})
	t.Run("kategori expense milik sendiri diterima", func(t *testing.T) {
		status, r := budget(owner.Token, food)
		expectStatus(t, status, http.StatusCreated, r)
	})
	t.Run("budget ganda di bulan yang sama ditolak", func(t *testing.T) {
		status, r := budget(owner.Token, food)
		expectStatus(t, status, http.StatusBadRequest, r)
	})
}
