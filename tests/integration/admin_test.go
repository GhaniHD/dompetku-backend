package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const testAdminEmail = "admin@test.dompetku"

var adminPassword = map[string]any{"password": "rahasia123"}

// adminUser memakai email yang terdaftar di ADMIN_EMAILS (lihat main_test.go).
func adminUser(t *testing.T) testUser {
	t.Helper()
	if token, ok := tryLogin(testAdminEmail, "rahasia123"); ok {
		return testUser{Email: testAdminEmail, Token: token}
	}
	status, r := call(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name": "Admin", "email": testAdminEmail, "password": "rahasia123",
	})
	expectStatus(t, status, http.StatusCreated, r)
	data := decode[struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		User   struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"user"`
	}](t, r)
	if data.Status != "active" || data.User.Role != "admin" || data.Token == "" {
		t.Fatalf("email ADMIN_EMAILS harus langsung admin aktif, dapat %+v", data)
	}
	return testUser{ID: data.User.ID, Email: testAdminEmail, Token: data.Token}
}

func tryLogin(email, password string) (string, bool) {
	status, out, err := tryCall(http.MethodPost, "/api/auth/login", "", map[string]any{"email": email, "password": password})
	return out.Token, err == nil && status == http.StatusOK
}

func adminCall(t *testing.T, admin testUser, method, path string, body any, want int) apiResponse {
	t.Helper()
	status, r := call(t, method, "/api/admin"+path, admin.Token, body)
	expectStatus(t, status, want, r)
	return r
}

func TestPendingUserCannotLoginUntilApproved(t *testing.T) {
	admin := adminUser(t)
	u := registerPending(t, "Calon")

	status, r := call(t, http.MethodPost, "/api/auth/login", "", map[string]any{"email": u.Email, "password": "rahasia123"})
	expectStatus(t, status, http.StatusForbidden, r)
	if !strings.Contains(r.Message, "menunggu persetujuan") {
		t.Fatalf("pesan login pending %q", r.Message)
	}

	users := decode[[]struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}](t, adminCall(t, admin, http.MethodGet, "/users?status=pending", nil, http.StatusOK))
	found := false
	for _, x := range users {
		found = found || x.ID == u.ID
	}
	if !found {
		t.Fatal("pendaftar tidak muncul di daftar pending admin")
	}

	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/approve", nil, http.StatusOK)
	token := login(t, u.Email, "rahasia123")
	status, r = call(t, http.MethodGet, "/api/wallets", token, nil)
	expectStatus(t, status, http.StatusOK, r)

	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/approve", nil, http.StatusConflict)
}

func TestRejectedUserStaysLockedOut(t *testing.T) {
	admin := adminUser(t)
	u := registerPending(t, "Ditolak")
	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/reject", map[string]any{"reason": "tidak dikenal"}, http.StatusOK)

	status, r := call(t, http.MethodPost, "/api/auth/login", "", map[string]any{"email": u.Email, "password": "rahasia123"})
	expectStatus(t, status, http.StatusForbidden, r)
	if !strings.Contains(r.Message, "ditolak") {
		t.Fatalf("pesan login ditolak %q", r.Message)
	}
}

func TestInviteActivatesImmediately(t *testing.T) {
	admin := adminUser(t)
	inv := decode[struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}](t, adminCall(t, admin, http.MethodPost, "/invites", map[string]any{"note": "keluarga", "max_uses": 2, "valid_days": 3}, http.StatusCreated))

	status, r := call(t, http.MethodGet, "/api/auth/invite/"+strings.ToLower(inv.Code), "", nil)
	expectStatus(t, status, http.StatusOK, r)
	if !decode[struct {
		Valid bool `json:"valid"`
	}](t, r).Valid {
		t.Fatal("undangan baru harus valid")
	}

	register := func(code string) (int, apiResponse) {
		return call(t, http.MethodPost, "/api/auth/register", "", map[string]any{
			"name": "Undangan", "email": fmt.Sprintf("inv-%s@test.dompetku", uuid.NewString()[:8]), "password": "rahasia123", "invite_code": code,
		})
	}
	for i := 0; i < 2; i++ {
		status, r := register(inv.Code)
		expectStatus(t, status, http.StatusCreated, r)
		data := decode[struct {
			Status string `json:"status"`
			Token  string `json:"token"`
		}](t, r)
		if data.Status != "active" || data.Token == "" {
			t.Fatalf("pendaftar lewat undangan harus langsung aktif, dapat %q", data.Status)
		}
	}

	status, r = register(inv.Code)
	expectStatus(t, status, http.StatusBadRequest, r)

	inv2 := decode[struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}](t, adminCall(t, admin, http.MethodPost, "/invites", map[string]any{}, http.StatusCreated))
	adminCall(t, admin, http.MethodDelete, "/invites/"+inv2.ID, nil, http.StatusOK)
	status, r = register(inv2.Code)
	expectStatus(t, status, http.StatusBadRequest, r)

	if n := countRows(t, `SELECT used_count FROM invites WHERE id = ?`, inv.ID); n != 2 {
		t.Fatalf("used_count %d, ingin 2", n)
	}
}

func TestSuspendKillsActiveSession(t *testing.T) {
	admin := adminUser(t)
	u := newUser(t)

	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/suspend", map[string]any{"reason": "uji"}, http.StatusOK)
	status, r := call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)
	status, r = call(t, http.MethodPost, "/api/auth/login", "", map[string]any{"email": u.Email, "password": "rahasia123"})
	expectStatus(t, status, http.StatusForbidden, r)

	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/activate", nil, http.StatusOK)
	token := login(t, u.Email, "rahasia123")
	status, r = call(t, http.MethodGet, "/api/wallets", token, nil)
	expectStatus(t, status, http.StatusOK, r)
}

func TestResetLinkChangesPasswordOnce(t *testing.T) {
	admin := adminUser(t)
	u := newUser(t)

	link := decode[struct {
		Token string `json:"token"`
	}](t, adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/reset-link", adminPassword, http.StatusCreated))

	status, r := call(t, http.MethodPost, "/api/auth/reset-password", "", map[string]any{"token": link.Token, "password": "pendek"})
	expectStatus(t, status, http.StatusBadRequest, r)

	status, r = call(t, http.MethodPost, "/api/auth/reset-password", "", map[string]any{"token": link.Token, "password": "rahasiaBaru1"})
	expectStatus(t, status, http.StatusOK, r)

	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)
	login(t, u.Email, "rahasiaBaru1")

	status, r = call(t, http.MethodPost, "/api/auth/reset-password", "", map[string]any{"token": link.Token, "password": "rahasiaLagi2"})
	expectStatus(t, status, http.StatusBadRequest, r)
}

func TestAdminEndpointsAreProtected(t *testing.T) {
	admin := adminUser(t)
	u := newUser(t)

	for _, path := range []string{"/summary", "/users", "/invites", "/logs"} {
		status, r := call(t, http.MethodGet, "/api/admin"+path, u.Token, nil)
		expectStatus(t, status, http.StatusForbidden, r)
	}
	status, r := call(t, http.MethodPost, "/api/admin/users/"+u.ID+"/suspend", u.Token, nil)
	expectStatus(t, status, http.StatusForbidden, r)

	adminID := decode[struct {
		ID string `json:"id"`
	}](t, func() apiResponse { _, r := call(t, http.MethodGet, "/api/profile", admin.Token, nil); return r }()).ID
	adminCall(t, admin, http.MethodPost, "/users/"+adminID+"/suspend", nil, http.StatusForbidden)
	adminCall(t, admin, http.MethodDelete, "/users/"+adminID, adminPassword, http.StatusForbidden)

	adminCall(t, admin, http.MethodDelete, "/users/"+u.ID, adminPassword, http.StatusOK)
	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)

	logs := decode[[]struct {
		Action string `json:"action"`
	}](t, adminCall(t, admin, http.MethodGet, "/logs", nil, http.StatusOK))
	if len(logs) == 0 || logs[0].Action != "delete" {
		t.Fatalf("log admin terbaru %v, ingin delete", logs)
	}

	sum := decode[struct {
		TotalUsers int64 `json:"total_users"`
	}](t, adminCall(t, admin, http.MethodGet, "/summary", nil, http.StatusOK))
	if sum.TotalUsers == 0 {
		t.Fatal("ringkasan admin kosong")
	}
}
