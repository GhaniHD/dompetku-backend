package integration_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRegisterNormalizesEmail(t *testing.T) {
	raw := "  User-" + uuid.NewString()[:8] + "@Test.DOMPETKU "
	status, r := call(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name": "Tester", "email": raw, "password": "rahasia123",
	})
	expectStatus(t, status, http.StatusCreated, r)

	want := strings.ToLower(strings.TrimSpace(raw))
	data := decode[struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}](t, r)
	if data.User.Email != want {
		t.Fatalf("email tersimpan %q, ingin %q", data.User.Email, want)
	}
	approveDirect(t, data.User.ID)

	status, r = call(t, http.MethodPost, "/api/auth/login", "", map[string]any{
		"email": strings.ToUpper(want), "password": "rahasia123",
	})
	expectStatus(t, status, http.StatusOK, r)
	if r.Token == "" {
		t.Fatal("login tidak mengembalikan token")
	}
}

func TestLoginUnknownEmailReturns401(t *testing.T) {
	status, r := call(t, http.MethodPost, "/api/auth/login", "", map[string]any{
		"email": "tidak-ada-" + uuid.NewString()[:8] + "@test.dompetku", "password": "rahasia123",
	})
	expectStatus(t, status, http.StatusUnauthorized, r)
}

func TestLoginWrongPasswordReturns401(t *testing.T) {
	u := newUser(t)
	status, r := call(t, http.MethodPost, "/api/auth/login", "", map[string]any{
		"email": u.Email, "password": "salah-password",
	})
	expectStatus(t, status, http.StatusUnauthorized, r)
}

func TestRegisterValidation(t *testing.T) {
	existing := newUser(t)

	cases := []struct {
		name  string
		email string
		pass  string
	}{
		{"email sudah terdaftar", existing.Email, "rahasia123"},
		{"email sudah terdaftar beda huruf", strings.ToUpper(existing.Email), "rahasia123"},
		{"format email salah", "bukan-email", "rahasia123"},
		{"password terlalu pendek", "pendek-" + uuid.NewString()[:8] + "@test.dompetku", "1234567"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, r := call(t, http.MethodPost, "/api/auth/register", "", map[string]any{
				"name": "X", "email": tc.email, "password": tc.pass,
			})
			expectStatus(t, status, http.StatusBadRequest, r)
		})
	}
}

func TestProtectedRoutesRequireValidToken(t *testing.T) {
	cases := map[string]string{
		"tanpa token":     "",
		"token sembarang": "bukan.jwt.valid",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			status, r := call(t, http.MethodGet, "/api/wallets", token, nil)
			expectStatus(t, status, http.StatusUnauthorized, r)
		})
	}
}

func TestDeletedAccountCannotUseTokenOrReRegister(t *testing.T) {
	u := newUser(t)

	status, r := call(t, http.MethodDelete, "/api/profile", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)

	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)

	status, r = call(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name": "Lagi", "email": u.Email, "password": "rahasia123",
	})
	expectStatus(t, status, http.StatusBadRequest, r)
	if strings.Contains(strings.ToLower(r.Message), "duplicate") {
		t.Fatalf("error database bocor ke client: %q", r.Message)
	}
}

func TestChangePasswordWrongOldPasswordIsBadRequest(t *testing.T) {
	u := newUser(t)

	status, r := call(t, http.MethodPut, "/api/profile/password", u.Token, map[string]any{
		"old_password": "bukan-password-lama", "new_password": "rahasia456",
	})
	expectStatus(t, status, http.StatusBadRequest, r)

	status, r = call(t, http.MethodPut, "/api/profile/password", u.Token, map[string]any{
		"old_password": "rahasia123", "new_password": "rahasia456",
	})
	expectStatus(t, status, http.StatusOK, r)
	fresh := decode[struct {
		Token string `json:"token"`
	}](t, r).Token

	status, r = call(t, http.MethodGet, "/api/wallets", u.Token, nil)
	expectStatus(t, status, http.StatusUnauthorized, r)
	status, r = call(t, http.MethodGet, "/api/wallets", fresh, nil)
	expectStatus(t, status, http.StatusOK, r)

	status, r = call(t, http.MethodPost, "/api/auth/login", "", map[string]any{"email": u.Email, "password": "rahasia456"})
	expectStatus(t, status, http.StatusOK, r)
}
