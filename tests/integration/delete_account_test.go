package integration_test

import (
	"net/http"
	"os"
	"testing"
)

func seedFinance(t *testing.T, u testUser) {
	t.Helper()
	wallet := createWallet(t, u, "Tunai", 300000)
	food := createCategory(t, u, "Makan", "expense")
	createTransaction(t, u, wallet, food, 25000, "expense")
	call(t, http.MethodPost, "/api/budgets", u.Token, map[string]any{"category_id": food, "amount": 500000, "month": 10, "year": 2026})
}

func expectNoTrace(t *testing.T, userID string) {
	t.Helper()
	if n := countRows(t, "SELECT COUNT(*) FROM users WHERE id = ?", userID); n != 0 {
		t.Fatalf("baris akun masih ada (%d)", n)
	}
	for table, n := range dataCounts(t, userID) {
		if n != 0 {
			t.Errorf("%s masih menyimpan %d baris", table, n)
		}
	}
	if n := countRows(t, "SELECT COUNT(*) FROM notifications WHERE user_id = ?", userID); n != 0 {
		t.Errorf("notifikasi masih ada (%d)", n)
	}
}

func registerAgain(t *testing.T, email string) {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/auth/register", "", map[string]any{"name": "Daftar Lagi", "email": email, "password": "rahasia123"})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("daftar ulang dengan email yang sama ditolak: %d %s", status, r.Message)
	}
}

func TestDeleteOwnAccountIsPermanent(t *testing.T) {
	u := newUser(t)
	seedFinance(t, u)

	t.Run("tanpa password ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodDelete, "/api/profile", u.Token, nil)
		expectStatus(t, status, http.StatusBadRequest, r)
	})
	t.Run("password salah ditolak", func(t *testing.T) {
		status, r := call(t, http.MethodDelete, "/api/profile", u.Token, map[string]any{"password": "salah-sekali"})
		expectStatus(t, status, http.StatusForbidden, r)
		if n := countRows(t, "SELECT COUNT(*) FROM transactions WHERE user_id = ?", u.ID); n != 1 {
			t.Fatalf("data berubah setelah password salah: %d transaksi", n)
		}
	})

	status, r := call(t, http.MethodDelete, "/api/profile", u.Token, map[string]any{"password": "rahasia123"})
	expectStatus(t, status, http.StatusOK, r)
	expectNoTrace(t, u.ID)

	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)

	registerAgain(t, u.Email)
	if n := countRows(t, "SELECT COUNT(*) FROM wallets w JOIN users u ON u.id = w.user_id WHERE LOWER(u.email) = LOWER(?)", u.Email); n != 0 {
		t.Fatalf("akun baru mewarisi %d dompet lama", n)
	}
}

func TestAdminCannotDeleteOwnAccountFromProfile(t *testing.T) {
	admin := adminUser(t)
	status, r := call(t, http.MethodDelete, "/api/profile", admin.Token, map[string]any{"password": "rahasia123"})
	expectStatus(t, status, http.StatusForbidden, r)
}

func TestAdminDeleteIsPermanent(t *testing.T) {
	u := newUser(t)
	seedFinance(t, u)
	admin := adminUser(t)

	adminCall(t, admin, http.MethodDelete, "/users/"+u.ID, adminPassword, http.StatusOK)
	expectNoTrace(t, u.ID)

	logs := decode[[]struct {
		Action string `json:"action"`
		Detail string `json:"detail"`
	}](t, adminCall(t, admin, http.MethodGet, "/logs", nil, http.StatusOK))
	if len(logs) == 0 || logs[0].Action != "delete" {
		t.Fatalf("log admin terbaru %v", logs)
	}
	if logs[0].Detail == u.Email || logs[0].Detail != u.Email[:2]+"***"+u.Email[len(u.Email)-len("@test.dompetku"):] {
		t.Fatalf("email di log harus disamarkan, dapat %q", logs[0].Detail)
	}

	registerAgain(t, u.Email)
}

func TestMigrationPurgesSoftDeletedUsers(t *testing.T) {
	u := newUser(t)
	seedFinance(t, u)
	if err := gdb.Exec("UPDATE users SET deleted_at = NOW() WHERE id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	keep := newUser(t)
	seedFinance(t, keep)

	sql, err := os.ReadFile("../../migrations/015_purge_deleted_users.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Exec(string(sql)).Error; err != nil {
		t.Fatalf("migrasi 015: %v", err)
	}
	expectNoTrace(t, u.ID)
	if n := countRows(t, "SELECT COUNT(*) FROM transactions WHERE user_id = ?", keep.ID); n != 1 {
		t.Fatalf("akun aktif ikut terhapus: %d transaksi", n)
	}
	registerAgain(t, u.Email)
}
