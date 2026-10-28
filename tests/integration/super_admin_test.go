package integration_test

import (
	"net/http"
	"testing"
	"time"
)

type activityRow struct {
	UserID *string `json:"user_id"`
	Email  string  `json:"email"`
	Event  string  `json:"event"`
	Detail string  `json:"detail"`
	Device string  `json:"device"`
}

func activityFor(t *testing.T, admin testUser, query string) []activityRow {
	t.Helper()
	return decode[[]activityRow](t, adminCall(t, admin, http.MethodGet, "/activity"+query, nil, http.StatusOK))
}

func hasEvent(rows []activityRow, event string) bool {
	for _, r := range rows {
		if r.Event == event {
			return true
		}
	}
	return false
}

func TestAdminCannotUseFinanceEndpoints(t *testing.T) {
	admin := adminUser(t)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/wallets"},
		{http.MethodGet, "/api/transactions"},
		{http.MethodGet, "/api/budgets/current"},
		{http.MethodPost, "/api/backup/export"},
		{http.MethodGet, "/api/reports/yearly?year=2026"},
	} {
		status, r := call(t, c.method, c.path, admin.Token, map[string]any{})
		if status != http.StatusForbidden {
			t.Errorf("%s %s: status %d (%s), ingin 403", c.method, c.path, status, r.Message)
		}
	}
	for _, path := range []string{"/api/profile", "/api/notifications", "/api/announcements"} {
		status, r := call(t, http.MethodGet, path, admin.Token, nil)
		expectStatus(t, status, http.StatusOK, r)
	}
}

func TestLoginActivityIsRecorded(t *testing.T) {
	u := newUser(t)
	if _, ok := tryLogin(u.Email, "salah-sekali"); ok {
		t.Fatal("login dengan password salah berhasil")
	}
	tryLogin("tidak-ada-"+u.Email, "rahasia123")
	u.Token = login(t, u.Email, "rahasia123")

	status, r := call(t, http.MethodPost, "/api/auth/logout", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)

	admin := adminUser(t)
	rows := activityFor(t, admin, "?user_id="+u.ID)
	for _, e := range []string{"login", "login_failed", "logout"} {
		if !hasEvent(rows, e) {
			t.Errorf("kejadian %s tidak tercatat: %+v", e, rows)
		}
	}
	unknown := activityFor(t, admin, "?event=login_failed&q=tidak-ada-")
	if len(unknown) == 0 || unknown[0].UserID != nil || unknown[0].Detail != "email tidak terdaftar" {
		t.Fatalf("login gagal email tak dikenal: %+v", unknown)
	}

	status, r = call(t, http.MethodGet, "/api/admin/activity", u.Token, nil)
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		t.Fatalf("pengguna biasa bisa membuka log: %d", status)
	}
}

func TestOnlinePresenceAndSummary(t *testing.T) {
	u := newUser(t)
	status, r := call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)

	admin := adminUser(t)
	online := decode[[]struct {
		ID           string `json:"id"`
		Online       bool   `json:"online"`
		LastSeenAt   string `json:"last_seen_at"`
		Wallets      int64  `json:"wallets"`
		Transactions int64  `json:"transactions"`
	}](t, adminCall(t, admin, http.MethodGet, "/users?online=true", nil, http.StatusOK))
	found := false
	for _, x := range online {
		found = found || (x.ID == u.ID && x.Online && x.LastSeenAt != "")
	}
	if !found {
		t.Fatalf("pengguna yang baru aktif tidak tercatat online: %+v", online)
	}

	sum := decode[struct {
		OnlineNow     int64                   `json:"online_now"`
		Online24h     []struct{ Users int64 } `json:"online_24h"`
		Registrations []struct{ Count int64 } `json:"registrations"`
	}](t, adminCall(t, admin, http.MethodGet, "/summary", nil, http.StatusOK))
	if sum.OnlineNow < 1 || len(sum.Online24h) != 24 || len(sum.Registrations) != 8 {
		t.Fatalf("ringkasan admin %+v", sum)
	}
	if sum.Online24h[23].Users < 1 {
		t.Fatalf("jam ini harus punya minimal 1 pengguna online: %+v", sum.Online24h)
	}

	call(t, http.MethodPost, "/api/auth/logout", u.Token, nil)
	detail := decode[struct {
		User struct {
			Online bool `json:"online"`
		} `json:"user"`
		Events []activityRow `json:"events"`
	}](t, adminCall(t, admin, http.MethodGet, "/users/"+u.ID, nil, http.StatusOK))
	if detail.User.Online || !hasEvent(detail.Events, "logout") {
		t.Fatalf("setelah keluar: %+v", detail)
	}
}

func TestForceLogoutNeedsAdminPassword(t *testing.T) {
	u := newUser(t)
	admin := adminUser(t)

	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/logout", map[string]any{"password": "bukan-ini"}, http.StatusForbidden)
	adminCall(t, admin, http.MethodDelete, "/users/"+u.ID, map[string]any{"password": "bukan-ini"}, http.StatusForbidden)
	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/reset-link", nil, http.StatusBadRequest)
	status, r := call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)

	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/logout", adminPassword, http.StatusOK)
	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)
	if r.Code != "session_ended" {
		t.Fatalf("kode sesi %q", r.Code)
	}
	if !hasEvent(activityFor(t, admin, "?user_id="+u.ID), "forced_logout") {
		t.Fatal("paksa keluar tidak tercatat")
	}
	login(t, u.Email, "rahasia123")
}

func TestAdminSessionExpiresAfterIdle(t *testing.T) {
	admin := adminUser(t)
	if err := gdb.Exec("UPDATE users SET last_seen_at = ? WHERE email = ?", time.Now().Add(-31*time.Minute), testAdminEmail).Error; err != nil {
		t.Fatal(err)
	}
	status, r := call(t, http.MethodGet, "/api/admin/summary", admin.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)
	if r.Code != "admin_idle" {
		t.Fatalf("kode %q", r.Code)
	}
	admin = adminUser(t)
	adminCall(t, admin, http.MethodGet, "/summary", nil, http.StatusOK)

	u := newUser(t)
	gdb.Exec("UPDATE users SET last_seen_at = ? WHERE id = ?", time.Now().Add(-2*time.Hour), u.ID)
	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
}

func TestAnnouncements(t *testing.T) {
	admin := adminUser(t)
	u := newUser(t)

	status, r := call(t, http.MethodPost, "/api/admin/announcements", u.Token, map[string]any{"message": "halo"})
	expectStatus(t, status, http.StatusForbidden, r)
	adminCall(t, admin, http.MethodPost, "/announcements", map[string]any{"message": "lama", "ends_at": time.Now().Add(-time.Hour)}, http.StatusBadRequest)

	created := decode[struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
	}](t, adminCall(t, admin, http.MethodPost, "/announcements", map[string]any{"message": "Pemeliharaan server Sabtu 22.00", "level": "warning"}, http.StatusCreated))
	if !created.Active {
		t.Fatal("pengumuman baru tidak aktif")
	}

	active := func() []struct {
		ID    string `json:"id"`
		Level string `json:"level"`
	} {
		status, r := call(t, http.MethodGet, "/api/announcements", u.Token, nil)
		expectStatus(t, status, http.StatusOK, r)
		return decode[[]struct {
			ID    string `json:"id"`
			Level string `json:"level"`
		}](t, r)
	}
	list := active()
	if len(list) == 0 || list[0].ID != created.ID || list[0].Level != "warning" {
		t.Fatalf("pengguna tidak melihat pengumuman: %+v", list)
	}

	adminCall(t, admin, http.MethodPost, "/announcements/"+created.ID+"/end", nil, http.StatusOK)
	adminCall(t, admin, http.MethodPost, "/announcements/"+created.ID+"/end", nil, http.StatusNotFound)
	for _, a := range active() {
		if a.ID == created.ID {
			t.Fatal("pengumuman yang diakhiri masih tampil")
		}
	}
}

func TestSuspiciousLoginsAndRetention(t *testing.T) {
	u := newUser(t)
	for i := 0; i < 5; i++ {
		tryLogin(u.Email, "tebakan-salah")
	}
	admin := adminUser(t)
	sum := decode[struct {
		LoginFailed24h int64 `json:"login_failed_24h"`
		Suspicious     []struct {
			Email    string `json:"email"`
			Attempts int64  `json:"attempts"`
		} `json:"suspicious"`
	}](t, adminCall(t, admin, http.MethodGet, "/summary", nil, http.StatusOK))
	found := false
	for _, s := range sum.Suspicious {
		found = found || (s.Email == u.Email && s.Attempts >= 5)
	}
	if !found || sum.LoginFailed24h < 5 {
		t.Fatalf("login mencurigakan tidak terdeteksi: %+v", sum)
	}

	old := time.Now().Add(-8 * 24 * time.Hour)
	gdb.Exec("UPDATE auth_events SET created_at = ? WHERE user_id = ?", old, u.ID)
	gdb.Exec("INSERT INTO presence_hours (hour, user_id) VALUES (date_trunc('hour', CAST(? AS timestamptz)), ?)", old, u.ID)
	if err := activitySvc.Purge(time.Now()); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM auth_events WHERE user_id = ?", u.ID); n != 0 {
		t.Fatalf("log lebih dari 7 hari masih ada: %d", n)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM presence_hours WHERE user_id = ? AND hour < now() - interval '7 days'", u.ID); n != 0 {
		t.Fatalf("data online lama masih ada: %d", n)
	}
}
