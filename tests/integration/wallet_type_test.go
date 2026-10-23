package integration_test

import (
	"net/http"
	"testing"
)

type walletResult struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Balance    float64 `json:"balance"`
	IsArchived bool    `json:"is_archived"`
}

type adjustmentResult struct {
	Amount          float64 `json:"amount"`
	PreviousBalance float64 `json:"previous_balance"`
	NewBalance      float64 `json:"new_balance"`
	Note            string  `json:"note"`
}

func adjust(t *testing.T, u testUser, walletID string, actual float64) (int, apiResponse) {
	t.Helper()
	return call(t, http.MethodPost, "/api/wallets/"+walletID+"/adjust", u.Token, map[string]any{
		"actual_balance": actual, "note": "cocokkan dengan aplikasi bank", "date": testDate,
	})
}

func listWallets(t *testing.T, u testUser, query string) []walletResult {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/wallets"+query, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[[]walletResult](t, r)
}

func TestNonCreditWalletCannotGoNegative(t *testing.T) {
	u := newUser(t)
	cash := createWallet(t, u, "Cash", 10_000)
	food := createCategory(t, u, "Makan", "expense")
	salary := createCategory(t, u, "Gaji", "income")

	t.Run("expense melebihi saldo", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/transactions", u.Token, txBody(cash, food, 15_000, "expense"))
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	t.Run("expense tepat sebesar saldo diizinkan", func(t *testing.T) {
		tx := createTransaction(t, u, cash, food, 10_000, "expense")
		expectBalance(t, u, cash, 0)
		status, r := call(t, http.MethodDelete, "/api/transactions/"+tx, u.Token, nil)
		expectStatus(t, status, http.StatusOK, r)
	})

	t.Run("menaikkan nominal expense sampai minus", func(t *testing.T) {
		tx := createTransaction(t, u, cash, food, 5_000, "expense")
		status, r := call(t, http.MethodPut, "/api/transactions/"+tx, u.Token, map[string]any{"amount": 20_000})
		expectStatus(t, status, http.StatusBadRequest, r)
		expectBalance(t, u, cash, 5_000)
	})

	t.Run("menghapus income yang sudah terpakai", func(t *testing.T) {
		income := createTransaction(t, u, cash, salary, 100_000, "income")
		createTransaction(t, u, cash, food, 100_000, "expense")
		status, r := call(t, http.MethodDelete, "/api/transactions/"+income, u.Token, nil)
		expectStatus(t, status, http.StatusBadRequest, r)
		expectBalance(t, u, cash, 5_000)
	})

	expectReconciled(t, u)
}

func TestCreditCardCanGoNegative(t *testing.T) {
	u := newUser(t)
	bank := createTypedWallet(t, u, "BCA", "bank", 2_000_000)
	card := createTypedWallet(t, u, "Kartu Kredit", "credit_card", 0)
	shopping := createCategory(t, u, "Belanja", "expense")

	createTransaction(t, u, card, shopping, 750_000, "expense")
	expectBalance(t, u, card, -750_000)

	createTransfer(t, u, bank, card, 750_000, 0)
	expectBalance(t, u, card, 0)
	expectBalance(t, u, bank, 1_250_000)
	expectReconciled(t, u)
}

func TestOpeningBalanceAndTypeRules(t *testing.T) {
	u := newUser(t)

	t.Run("saldo awal minus untuk cash ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/wallets", u.Token, map[string]any{"name": "Cash", "type": "cash", "balance": -1})
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	t.Run("tipe tidak dikenal ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/wallets", u.Token, map[string]any{"name": "X", "type": "kripto", "balance": 0})
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	t.Run("tanpa tipe menjadi cash", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/wallets", u.Token, map[string]any{"name": "Dompet", "balance": 0})
		expectStatus(t, status, http.StatusCreated, r)
		if got := decode[walletResult](t, r).Type; got != "cash" {
			t.Fatalf("tipe default %q, ingin cash", got)
		}
	})

	card := createTypedWallet(t, u, "Kartu Kredit", "credit_card", -300_000)

	t.Run("kartu kredit minus tidak bisa diubah jadi bank", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/wallets/"+card, u.Token, map[string]any{"type": "bank"})
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	expectReconciled(t, u)
}

func TestAdjustBalance(t *testing.T) {
	u := newUser(t)
	bank := createTypedWallet(t, u, "BCA", "bank", 1_000_000)
	food := createCategory(t, u, "Makan", "expense")
	createTransaction(t, u, bank, food, 50_000, "expense")
	before := reportOctober(t, u)

	status, r := adjust(t, u, bank, 937_500)
	expectStatus(t, status, http.StatusCreated, r)
	got := decode[adjustmentResult](t, r)
	if got.Amount != -12_500 || got.PreviousBalance != 950_000 || got.NewBalance != 937_500 {
		t.Fatalf("penyesuaian salah: %+v", got)
	}
	expectBalance(t, u, bank, 937_500)

	status, r = adjust(t, u, bank, 1_000_000)
	expectStatus(t, status, http.StatusCreated, r)
	expectBalance(t, u, bank, 1_000_000)

	t.Run("saldo sama ditolak", func(t *testing.T) {
		status, r := adjust(t, u, bank, 1_000_000)
		expectStatus(t, status, http.StatusBadRequest, r)
	})
	t.Run("bank tidak boleh disesuaikan ke minus", func(t *testing.T) {
		status, r := adjust(t, u, bank, -1)
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	status, r = call(t, http.MethodGet, "/api/wallets/"+bank+"/adjustments", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if n := len(decode[[]adjustmentResult](t, r)); n != 2 {
		t.Fatalf("riwayat penyesuaian %d, ingin 2", n)
	}

	if after := reportOctober(t, u); after.TotalIncome != before.TotalIncome || after.TotalExpense != before.TotalExpense {
		t.Fatal("penyesuaian saldo tidak boleh masuk laporan pemasukan/pengeluaran")
	}
	expectReconciled(t, u)
}

func TestPutBalanceIsRecordedAsAdjustment(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Cash", 100_000)

	status, r := call(t, http.MethodPut, "/api/wallets/"+wallet, u.Token, map[string]any{"name": "Cash Harian", "balance": 80_000})
	expectStatus(t, status, http.StatusOK, r)

	status, r = call(t, http.MethodPut, "/api/wallets/"+wallet, u.Token, map[string]any{"name": "Cash Harian", "balance": 80_000})
	expectStatus(t, status, http.StatusOK, r)

	status, r = call(t, http.MethodGet, "/api/wallets/"+wallet+"/adjustments", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	adjustments := decode[[]adjustmentResult](t, r)
	if len(adjustments) != 1 || adjustments[0].Amount != -20_000 {
		t.Fatalf("ingin 1 penyesuaian -20000, dapat %+v", adjustments)
	}
	expectReconciled(t, u)
}

func TestDeleteWalletWithoutHistoryRemovesIt(t *testing.T) {
	u := newUser(t)
	wallet := createWallet(t, u, "Salah Buat", 25_000)

	status, r := call(t, http.MethodDelete, "/api/wallets/"+wallet, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if decode[struct {
		Archived bool `json:"archived"`
	}](t, r).Archived {
		t.Fatal("wallet tanpa riwayat seharusnya dihapus, bukan diarsipkan")
	}

	status, r = call(t, http.MethodGet, "/api/wallets/"+wallet, u.Token, nil)
	expectStatus(t, status, http.StatusNotFound, r)
}

func TestDeleteWalletWithHistoryArchives(t *testing.T) {
	u := newUser(t)
	old := createWallet(t, u, "Dompet Lama", 100_000)
	primary := createWallet(t, u, "Dompet Utama", 0)
	food := createCategory(t, u, "Makan", "expense")
	createTransaction(t, u, old, food, 30_000, "expense")
	reportBefore := reportOctober(t, u)

	t.Run("masih ada saldo", func(t *testing.T) {
		status, r := call(t, http.MethodDelete, "/api/wallets/"+old, u.Token, nil)
		expectStatus(t, status, http.StatusConflict, r)
	})

	createTransfer(t, u, old, primary, 70_000, 0)

	status, r := call(t, http.MethodDelete, "/api/wallets/"+old, u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if !decode[struct {
		Archived bool `json:"archived"`
	}](t, r).Archived {
		t.Fatal("wallet dengan riwayat seharusnya diarsipkan")
	}

	if active := listWallets(t, u, ""); len(active) != 1 || active[0].ID != primary {
		t.Fatalf("daftar dompet aktif salah: %+v", active)
	}
	if all := listWallets(t, u, "?include_archived=true"); len(all) != 2 {
		t.Fatalf("include_archived harus menampilkan 2 dompet, dapat %d", len(all))
	}

	if after := reportOctober(t, u); after.TotalExpense != reportBefore.TotalExpense {
		t.Fatal("riwayat transaksi dompet yang diarsipkan hilang dari laporan")
	}

	t.Run("dompet arsip tidak bisa dipakai transaksi baru", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/transactions", u.Token, txBody(old, food, 1_000, "expense"))
		expectStatus(t, status, http.StatusBadRequest, r)
	})
	t.Run("dompet arsip tidak bisa menerima transfer", func(t *testing.T) {
		status, r := call(t, http.MethodPost, "/api/transfers", u.Token, transferBody(primary, old, 1_000, 0))
		expectStatus(t, status, http.StatusBadRequest, r)
	})

	t.Run("diaktifkan lagi lalu bisa dipakai", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/wallets/"+old, u.Token, map[string]any{"is_archived": false})
		expectStatus(t, status, http.StatusOK, r)
		createTransfer(t, u, primary, old, 1_000, 0)
		expectBalance(t, u, old, 1_000)
	})

	t.Run("arsip lewat PUT wajib saldo 0", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/wallets/"+old, u.Token, map[string]any{"is_archived": true})
		expectStatus(t, status, http.StatusConflict, r)
	})

	expectReconciled(t, u)
}

func TestAdjustmentOwnership(t *testing.T) {
	owner := newUser(t)
	wallet := createWallet(t, owner, "Gopay", 100_000)
	other := newUser(t)

	status, r := adjust(t, other, wallet, 0)
	expectStatus(t, status, http.StatusNotFound, r)

	status, r = call(t, http.MethodGet, "/api/wallets/"+wallet+"/adjustments", other.Token, nil)
	expectStatus(t, status, http.StatusNotFound, r)

	expectBalance(t, owner, wallet, 100_000)
}
