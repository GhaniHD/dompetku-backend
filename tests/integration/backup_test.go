package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"testing"

	"dompetku/pkg/backupcrypt"
)

const backupPassword = "kunci-backup-123"

func exportBackup(t *testing.T, u testUser, password, backupPw string) (int, []byte, string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"password": password, "backup_password": backupPw})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/backup/export", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+u.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body, res.Header.Get("Content-Disposition")
}

func backupForm(t *testing.T, u testUser, path string, file []byte, fields map[string]string) (int, apiResponse) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("file", "backup.dpk")
	part.Write(file)
	for k, v := range fields {
		w.WriteField(k, v)
	}
	w.Close()
	req, _ := http.NewRequest(http.MethodPost, server.URL+path, &body)
	req.Header.Set("Authorization", "Bearer "+u.Token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out apiResponse
	json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

type walletSnapshot struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Balance float64 `json:"balance"`
}

func walletState(t *testing.T, u testUser) []walletSnapshot {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/wallets?include_archived=true", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	ws := decode[[]walletSnapshot](t, r)
	sort.Slice(ws, func(i, j int) bool { return ws[i].Name < ws[j].Name })
	return ws
}

func dataCounts(t *testing.T, userID string) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, table := range []string{"categories", "wallets", "transactions", "transfers", "balance_adjustments", "debts", "debt_movements", "goals", "budgets", "recurring_transactions"} {
		out[table] = countRows(t, "SELECT COUNT(*) FROM "+table+" WHERE user_id = ?", userID)
	}
	return out
}

func TestBackupAndRestore(t *testing.T) {
	a := newUser(t)
	bca := createTypedWallet(t, a, "BCA", "bank", 3_000_000)
	cash := createWallet(t, a, "Tunai", 200_000)
	savings := createTypedWallet(t, a, "Tabungan", "savings", 0)
	food := createCategory(t, a, "Makan", "expense")
	salary := createCategory(t, a, "Gaji", "income")
	old := createCategory(t, a, "Lama", "expense")

	incomeOn(t, a, bca, salary, 8_000_000, "2026-10-01")
	expenseOn(t, a, cash, old, 15_000, "2026-10-02")
	status, r := call(t, http.MethodDelete, "/api/categories/"+old, a.Token, nil)
	if status >= 300 {
		t.Logf("hapus kategori yang dipakai: %d %s", status, r.Message)
	}
	expenseOn(t, a, bca, food, 120_000, "2026-10-03")
	transferOn(t, a, bca, cash, 300_000, 2_500, "2026-10-04")
	transferOn(t, a, bca, savings, 1_000_000, 0, "2026-10-05")
	status, r = call(t, http.MethodPost, "/api/wallets/"+cash+"/adjust", a.Token, map[string]any{"actual_balance": 480_000, "date": "2026-10-06T09:00:00+07:00"})
	expectStatus(t, status, http.StatusCreated, r)
	debt := createDebt(t, a, map[string]any{"kind": "payable", "counterparty": "Budi", "amount": 500_000, "wallet_id": bca, "date": testDate})
	status, r = payDebt(t, a, debt.ID, map[string]any{"amount": 200_000, "wallet_id": bca, "date": testDate})
	expectStatus(t, status, http.StatusCreated, r)
	status, r = call(t, http.MethodPost, "/api/goals", a.Token, map[string]any{"name": "Liburan", "target_amount": 5_000_000, "wallet_id": savings})
	expectStatus(t, status, http.StatusCreated, r)
	goal := decode[idResponse](t, r).ID
	createBudgetFor(t, a, food, 1_000_000, 10, 2026)
	status, r = call(t, http.MethodPost, "/api/budgets", a.Token, map[string]any{"kind": "savings", "goal_id": goal, "amount": 750_000, "month": 10, "year": 2026})
	expectStatus(t, status, http.StatusCreated, r)
	createRecurring(t, a, recurringBody(bca, food, "expense", "monthly", 50_000, todayWIB().AddDate(0, 1, 0)))
	status, r = call(t, http.MethodPut, "/api/profile/income", a.Token, map[string]any{"income_type": "salary", "start_day": 25})
	expectStatus(t, status, http.StatusOK, r)

	t.Run("password salah ditolak", func(t *testing.T) {
		if status, _, _ := exportBackup(t, a, "bukan-password", backupPassword); status != http.StatusForbidden {
			t.Fatalf("password akun salah: %d", status)
		}
		if status, _, _ := exportBackup(t, a, "rahasia123", "pendek"); status != http.StatusBadRequest {
			t.Fatalf("password backup terlalu pendek: %d", status)
		}
	})

	status, file, disposition := exportBackup(t, a, "rahasia123", backupPassword)
	if status != http.StatusOK || !strings.Contains(disposition, ".dpk") {
		t.Fatalf("unduh backup %d %q", status, disposition)
	}
	if bytes.Contains(file, []byte("BCA")) || bytes.Contains(file, []byte("Liburan")) {
		t.Fatal("isi backup terbaca tanpa password")
	}

	b := newUser(t)
	createWallet(t, b, "Dompet lama B", 999_000)
	createCategory(t, b, "Kategori lama B", "expense")
	notifBefore := countRows(t, "SELECT COUNT(*) FROM notifications WHERE user_id = ?", b.ID)

	auth := map[string]string{"password": "rahasia123", "backup_password": backupPassword}
	t.Run("pratinjau", func(t *testing.T) {
		status, r := backupForm(t, b, "/api/backup/preview", file, auth)
		expectStatus(t, status, http.StatusOK, r)
		p := decode[struct {
			SourceEmail  string  `json:"source_email"`
			Wallets      int     `json:"wallets"`
			Transactions int     `json:"transactions"`
			Transfers    int     `json:"transfers"`
			Goals        int     `json:"goals"`
			Budgets      int     `json:"budgets"`
			TotalBalance float64 `json:"total_balance"`
		}](t, r)
		var total float64
		for _, w := range walletState(t, a) {
			total += w.Balance
		}
		if p.SourceEmail != a.Email || p.Wallets != 3 || p.Transfers != 2 || p.Goals != 1 || p.Budgets != 2 || p.TotalBalance != total {
			t.Fatalf("pratinjau %+v, saldo total ingin %v", p, total)
		}
		if countRows(t, "SELECT COUNT(*) FROM wallets WHERE user_id = ?", b.ID) != 1 {
			t.Fatal("pratinjau mengubah data")
		}
	})

	t.Run("ditolak bila salah", func(t *testing.T) {
		for name, tc := range map[string]struct {
			path   string
			file   []byte
			fields map[string]string
			want   int
		}{
			"password backup salah": {"/api/backup/preview", file, map[string]string{"password": "rahasia123", "backup_password": "salah-password"}, http.StatusBadRequest},
			"password akun salah":   {"/api/backup/preview", file, map[string]string{"password": "salah", "backup_password": backupPassword}, http.StatusForbidden},
			"bukan file backup":     {"/api/backup/preview", []byte("halo ini teks biasa, bukan backup"), auth, http.StatusBadRequest},
			"file diubah":           {"/api/backup/preview", append(append([]byte(nil), file[:len(file)-1]...), file[len(file)-1]^1), auth, http.StatusBadRequest},
			"tanpa ketik GANTI":     {"/api/backup/restore", file, auth, http.StatusBadRequest},
		} {
			if status, r := backupForm(t, b, tc.path, tc.file, tc.fields); status != tc.want {
				t.Errorf("%s: status %d (%s), ingin %d", name, status, r.Message, tc.want)
			}
		}
		if countRows(t, "SELECT COUNT(*) FROM wallets WHERE user_id = ?", b.ID) != 1 {
			t.Fatal("percobaan gagal mengubah data")
		}
	})

	before := dataCounts(t, a.ID)
	restore := map[string]string{"password": "rahasia123", "backup_password": backupPassword, "confirm": "GANTI"}

	t.Run("isi tidak konsisten dibatalkan utuh", func(t *testing.T) {
		plain, err := backupcrypt.Open(file, backupPassword)
		if err != nil {
			t.Fatal(err)
		}
		for name, mutate := range map[string]func(m map[string]any){
			"saldo dompet diubah": func(m map[string]any) {
				m["wallets"].([]any)[0].(map[string]any)["balance"] = 123.45
			},
			"referensi dompet hilang": func(m map[string]any) {
				m["wallets"] = m["wallets"].([]any)[1:]
			},
		} {
			var m map[string]any
			json.Unmarshal(plain, &m)
			mutate(m)
			raw, _ := json.Marshal(m)
			bad, _ := backupcrypt.Seal(raw, backupPassword)
			if status, r := backupForm(t, b, "/api/backup/restore", bad, restore); status != http.StatusBadRequest {
				t.Errorf("%s: status %d (%s), ingin 400", name, status, r.Message)
			}
			if n := countRows(t, "SELECT COUNT(*) FROM wallets WHERE user_id = ? AND name = 'Dompet lama B'", b.ID); n != 1 {
				t.Fatalf("%s: data lama akun tujuan hilang walau pemulihan gagal", name)
			}
		}
	})
	status, r = backupForm(t, b, "/api/backup/restore", file, restore)
	expectStatus(t, status, http.StatusOK, r)

	if got, want := walletState(t, b), walletState(t, a); !equalWallets(got, want) {
		t.Fatalf("dompet hasil pulihkan %+v, ingin %+v", got, want)
	}
	after := dataCounts(t, b.ID)
	for table, n := range before {
		if after[table] != n {
			t.Errorf("%s: %d baris setelah dipulihkan, ingin %d", table, after[table], n)
		}
	}
	if n := countRows(t, "SELECT COUNT(*) FROM wallets WHERE user_id = ? AND name = 'Dompet lama B'", b.ID); n != 0 {
		t.Fatal("data lama akun tujuan masih ada")
	}
	if n := countRows(t, "SELECT COUNT(*) FROM wallets w JOIN wallets x ON x.id = w.id WHERE w.user_id = ? AND x.user_id = ?", a.ID, b.ID); n != 0 {
		t.Fatal("ID dompet sama dengan akun asal")
	}
	if got := countRows(t, "SELECT COUNT(*) FROM notifications WHERE user_id = ?", b.ID); got != notifBefore+1 {
		t.Fatalf("notifikasi %d, ingin %d (lama tetap + 1 pemberitahuan pulihkan)", got, notifBefore+1)
	}
	status, r = call(t, http.MethodGet, "/api/profile", b.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if p := decode[incomeProfile](t, r); p.StartDay != 25 || p.Type != "salary" || !p.SetupDone {
		t.Fatalf("profil setelah pulihkan %+v", p)
	}
	status, r = call(t, http.MethodGet, "/api/budgets?month=10&year=2026", b.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	if plan := decode[budgetPlan](t, r); plan.SavingsPlanned != 750_000 || plan.TotalBudget != 1_000_000 {
		t.Fatalf("anggaran setelah pulihkan %+v", plan)
	}
	if len(walletState(t, a)) != 3 || dataCounts(t, a.ID)["transactions"] != before["transactions"] {
		t.Fatal("data akun asal ikut berubah")
	}

	t.Run("pulihkan ke akun sendiri", func(t *testing.T) {
		expenseOn(t, a, bca, food, 77_000, "2026-10-20")
		status, r := backupForm(t, a, "/api/backup/restore", file, restore)
		expectStatus(t, status, http.StatusOK, r)
		if got := dataCounts(t, a.ID)["transactions"]; got != before["transactions"] {
			t.Fatalf("transaksi setelah pulihkan %d, ingin %d (transaksi sesudah backup hilang)", got, before["transactions"])
		}
	})
}

func equalWallets(a, b []walletSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBackupRateLimit(t *testing.T) {
	limiter.SetEnabled(true)
	defer limiter.SetEnabled(false)
	u := newUser(t)
	statuses := map[int]int{}
	for i := 0; i < 6; i++ {
		status, _, _ := exportBackup(t, u, "rahasia123", backupPassword)
		statuses[status]++
	}
	if statuses[http.StatusOK] != 5 || statuses[http.StatusTooManyRequests] != 1 {
		t.Fatalf("unduh backup berulang %v, ingin 5 berhasil lalu 1 ditahan", statuses)
	}
}
