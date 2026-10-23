package integration_test

import (
	"net/http"
	"testing"
)

type transferResult struct {
	ID               string  `json:"id"`
	Amount           float64 `json:"amount"`
	Fee              float64 `json:"fee"`
	FeeTransactionID *string `json:"fee_transaction_id"`
	FromWallet       struct {
		Name string `json:"name"`
	} `json:"from_wallet"`
	ToWallet struct {
		Name string `json:"name"`
	} `json:"to_wallet"`
}

type monthlyReport struct {
	TotalIncome       float64 `json:"total_income"`
	TotalExpense      float64 `json:"total_expense"`
	ExpenseByCategory []struct {
		CategoryName string  `json:"category_name"`
		Total        float64 `json:"total"`
	} `json:"expense_by_category"`
}

func transferBody(from, to string, amount, fee float64) map[string]any {
	return map[string]any{
		"from_wallet_id": from, "to_wallet_id": to,
		"amount": amount, "fee": fee, "note": "tes", "date": testDate,
	}
}

func createTransfer(t *testing.T, u testUser, from, to string, amount, fee float64) transferResult {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/transfers", u.Token, transferBody(from, to, amount, fee))
	expectStatus(t, status, http.StatusCreated, r)
	return decode[transferResult](t, r)
}

func reportOctober(t *testing.T, u testUser) monthlyReport {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/reports?month=10&year=2026", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[monthlyReport](t, r)
}

func TestTransferWithFee(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 1_000_000)
	cash := createWallet(t, u, "Cash", 50_000)
	before := reportOctober(t, u)

	tf := createTransfer(t, u, gopay, cash, 500_000, 2_500)
	if tf.FromWallet.Name != "Gopay" || tf.ToWallet.Name != "Cash" || tf.FeeTransactionID == nil {
		t.Fatalf("respons transfer tidak lengkap: %+v", tf)
	}

	expectBalance(t, u, gopay, 497_500)
	expectBalance(t, u, cash, 550_000)

	after := reportOctober(t, u)
	if after.TotalIncome != before.TotalIncome {
		t.Fatalf("transfer menambah pemasukan di laporan: %.0f -> %.0f", before.TotalIncome, after.TotalIncome)
	}
	if diff := after.TotalExpense - before.TotalExpense; diff != 2_500 {
		t.Fatalf("pengeluaran bertambah %.0f, ingin hanya biaya admin 2500", diff)
	}
	found := false
	for _, c := range after.ExpenseByCategory {
		if c.CategoryName == "Biaya Admin" && c.Total == 2_500 {
			found = true
		}
	}
	if !found {
		t.Fatal("biaya admin tidak tercatat di kategori 'Biaya Admin'")
	}

	expectReconciled(t, u)
}

func TestTransferWithoutFeeCreatesNoTransaction(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 100_000)
	cash := createWallet(t, u, "Cash", 0)

	tf := createTransfer(t, u, gopay, cash, 40_000, 0)
	if tf.FeeTransactionID != nil {
		t.Fatal("transfer tanpa biaya tidak boleh membuat transaksi biaya admin")
	}
	if n := countRows(t, "SELECT COUNT(*) FROM transactions WHERE user_id = ?", u.ID); n != 0 {
		t.Fatalf("ada %d transaksi, ingin 0", n)
	}
	expectBalance(t, u, gopay, 60_000)
	expectBalance(t, u, cash, 40_000)
	expectReconciled(t, u)
}

func TestTransferValidation(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 100_000)
	cash := createWallet(t, u, "Cash", 10_000)

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"dompet asal sama dengan tujuan", transferBody(cash, cash, 1_000, 0), http.StatusBadRequest},
		{"saldo tidak cukup untuk nominal + biaya", transferBody(cash, gopay, 9_000, 1_500), http.StatusBadRequest},
		{"nominal negatif", transferBody(cash, gopay, -5, 0), http.StatusBadRequest},
		{"nominal nol", transferBody(cash, gopay, 0, 0), http.StatusBadRequest},
		{"biaya negatif", transferBody(cash, gopay, 1_000, -1), http.StatusBadRequest},
		{"dompet tujuan tidak ada", transferBody(cash, "00000000-0000-0000-0000-000000000001", 1_000, 0), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, r := call(t, http.MethodPost, "/api/transfers", u.Token, tc.body)
			expectStatus(t, status, tc.want, r)
		})
	}

	expectBalance(t, u, gopay, 100_000)
	expectBalance(t, u, cash, 10_000)
}

func TestTransferFeeTransactionIsProtected(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 100_000)
	cash := createWallet(t, u, "Cash", 0)
	tf := createTransfer(t, u, gopay, cash, 50_000, 2_500)

	status, r := call(t, http.MethodDelete, "/api/transactions/"+*tf.FeeTransactionID, u.Token, nil)
	expectStatus(t, status, http.StatusConflict, r)

	status, r = call(t, http.MethodPut, "/api/transactions/"+*tf.FeeTransactionID, u.Token, map[string]any{"amount": 1})
	expectStatus(t, status, http.StatusConflict, r)

	expectBalance(t, u, gopay, 47_500)
	expectReconciled(t, u)
}

func TestDeleteTransferReversesEverything(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 100_000)
	cash := createWallet(t, u, "Cash", 0)
	before := reportOctober(t, u)
	tf := createTransfer(t, u, gopay, cash, 50_000, 2_500)

	status, r := call(t, http.MethodDelete, "/api/transfers/"+tf.ID, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)

	expectBalance(t, u, gopay, 100_000)
	expectBalance(t, u, cash, 0)
	if after := reportOctober(t, u); after.TotalExpense != before.TotalExpense {
		t.Fatalf("biaya admin masih tercatat setelah transfer dihapus: %.0f", after.TotalExpense)
	}

	status, r = call(t, http.MethodGet, "/api/transfers/"+tf.ID, u.Token, nil)
	expectStatus(t, status, http.StatusNotFound, r)
	expectReconciled(t, u)
}

func TestTransferListFilter(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 100_000)
	cash := createWallet(t, u, "Cash", 100_000)
	bank := createWallet(t, u, "Bank", 100_000)
	createTransfer(t, u, gopay, cash, 1_000, 0)
	createTransfer(t, u, cash, bank, 1_000, 0)
	createTransfer(t, u, bank, gopay, 1_000, 0)

	status, r := call(t, http.MethodGet, "/api/transfers?wallet_id="+cash, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if got := len(decode[[]transferResult](t, r)); got != 2 {
		t.Fatalf("dapat %d transfer untuk dompet Cash, ingin 2", got)
	}
}

func TestTransferOwnership(t *testing.T) {
	owner := newUser(t)
	gopay := createWallet(t, owner, "Gopay", 100_000)
	cash := createWallet(t, owner, "Cash", 0)
	tf := createTransfer(t, owner, gopay, cash, 10_000, 0)

	other := newUser(t)
	otherWallet := createWallet(t, other, "Dompet Lain", 100)

	t.Run("dari dompet user lain", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/transfers", other.Token, transferBody(gopay, otherWallet, 1_000, 0))
		expectStatus(t, status, http.StatusNotFound, r)
	})
	t.Run("ke dompet user lain", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/transfers", other.Token, transferBody(otherWallet, gopay, 50, 0))
		expectStatus(t, status, http.StatusNotFound, r)
	})
	t.Run("lihat dan hapus transfer user lain", func(t *testing.T) {
		for _, m := range []string{http.MethodGet, http.MethodDelete} {
			status, r := call(t, m, "/api/transfers/"+tf.ID, other.Token, nil)
			expectStatus(t, status, http.StatusNotFound, r)
		}
	})

	expectBalance(t, owner, gopay, 90_000)
	expectBalance(t, owner, cash, 10_000)
	expectBalance(t, other, otherWallet, 100)
}

func TestConcurrentTransfersBothDirections(t *testing.T) {
	u := newUser(t)
	gopay := createWallet(t, u, "Gopay", 1_000_000)
	cash := createWallet(t, u, "Cash", 1_000_000)

	const n = 40
	statuses := callConcurrently(t, n, func(i int) (string, string, string, any) {
		if i%2 == 0 {
			return http.MethodPost, "/api/transfers", u.Token, transferBody(gopay, cash, 1_000, 100)
		}
		return http.MethodPost, "/api/transfers", u.Token, transferBody(cash, gopay, 1_000, 0)
	})
	expectAllStatus(t, statuses, http.StatusCreated)

	expectBalance(t, u, gopay, 1_000_000-(n/2)*100)
	expectBalance(t, u, cash, 1_000_000)
	expectReconciled(t, u)
}
