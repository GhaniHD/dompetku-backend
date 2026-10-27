package integration_test

import (
	"net/http"
	"testing"
)

type dailyProfile struct {
	DailyCategoryIDs []string `json:"daily_category_ids"`
}

func dailyCategories(t *testing.T, u testUser) []string {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/profile", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[dailyProfile](t, r).DailyCategoryIDs
}

func TestDailyCategories(t *testing.T) {
	u := newUser(t)
	other := newUser(t)
	food := createCategory(t, u, "Makan", "expense")
	fuel := createCategory(t, u, "Bensin", "expense")
	salary := createCategory(t, u, "Gaji", "income")
	foreign := createCategory(t, other, "Makan", "expense")
	wallet := createWallet(t, u, "Tunai", 500000)

	if got := dailyCategories(t, u); got == nil || len(got) != 0 {
		t.Fatalf("default harus daftar kosong, dapat %v", got)
	}

	t.Run("simpan dan duplikat dibuang", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/profile/daily-categories", u.Token, map[string]any{"category_ids": []string{food, fuel, food}})
		expectStatus(t, status, http.StatusOK, r)
		if got := decode[dailyProfile](t, r).DailyCategoryIDs; len(got) != 2 || got[0] != food || got[1] != fuel {
			t.Fatalf("pilihan tersimpan %v", got)
		}
		if got := dailyCategories(t, u); len(got) != 2 {
			t.Fatalf("GET profil %v", got)
		}
	})

	t.Run("ditolak", func(t *testing.T) {
		for name, ids := range map[string][]string{
			"kategori pemasukan": {food, salary},
			"kategori akun lain": {foreign},
			"kategori tidak ada": {"00000000-0000-0000-0000-000000000001"},
		} {
			status, r := call(t, http.MethodPut, "/api/profile/daily-categories", u.Token, map[string]any{"category_ids": ids})
			if status != http.StatusBadRequest {
				t.Errorf("%s: status %d %s", name, status, r.Message)
			}
		}
		status, _ := call(t, http.MethodPut, "/api/profile/daily-categories", u.Token, map[string]any{"category_ids": []string{"bukan-uuid"}})
		if status != http.StatusBadRequest {
			t.Errorf("id rusak: status %d", status)
		}
		if got := dailyCategories(t, u); len(got) != 2 {
			t.Fatalf("pilihan lama berubah setelah permintaan ditolak: %v", got)
		}
	})

	t.Run("transaksi membawa category_id", func(t *testing.T) {
		createTransaction(t, u, wallet, food, 25000, "expense")
		status, r := call(t, http.MethodGet, "/api/transactions", u.Token, nil)
		expectStatus(t, status, http.StatusOK, r)
		txs := decode[[]struct {
			CategoryID string `json:"category_id"`
		}](t, r)
		if len(txs) != 1 || txs[0].CategoryID != food {
			t.Fatalf("transaksi %+v", txs)
		}
	})

	t.Run("ikut backup dan dipetakan ke kategori baru", func(t *testing.T) {
		status, file, _ := exportBackup(t, u, "rahasia123", backupPassword)
		if status != http.StatusOK {
			t.Fatalf("export %d", status)
		}
		target := newUser(t)
		status, r := backupForm(t, target, "/api/backup/restore", file, map[string]string{"password": "rahasia123", "backup_password": backupPassword, "confirm": "GANTI"})
		expectStatus(t, status, http.StatusOK, r)

		got := dailyCategories(t, target)
		if len(got) != 2 {
			t.Fatalf("pilihan setelah pulihkan %v", got)
		}
		names := map[string]bool{}
		for _, id := range got {
			if id == food || id == fuel {
				t.Fatalf("masih memakai ID kategori akun asal: %v", got)
			}
			var name string
			if err := gdb.Raw("SELECT name FROM categories WHERE id = ? AND user_id = ?", id, target.ID).Scan(&name).Error; err != nil {
				t.Fatal(err)
			}
			names[name] = true
		}
		if !names["Makan"] || !names["Bensin"] {
			t.Fatalf("kategori hasil pemetaan %v", names)
		}
	})

	t.Run("kosongkan berarti semua anggaran", func(t *testing.T) {
		status, r := call(t, http.MethodPut, "/api/profile/daily-categories", u.Token, map[string]any{"category_ids": []string{}})
		expectStatus(t, status, http.StatusOK, r)
		if got := dailyCategories(t, u); len(got) != 0 {
			t.Fatalf("harus kosong, dapat %v", got)
		}
	})
}
