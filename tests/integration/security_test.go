package integration_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"testing"
)

const chromeWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"

func rawRequest(t *testing.T, method, path, token string, body io.Reader, headers map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, server.URL+path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

func loginAs(t *testing.T, email, password, userAgent string) (int, string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"email": email, "password": password})
	res, out := rawRequest(t, http.MethodPost, "/api/auth/login", "", bytes.NewReader(raw), map[string]string{
		"Content-Type": "application/json", "User-Agent": userAgent,
	})
	token, _ := out["token"].(string)
	return res.StatusCode, token
}

func TestSingleDeviceSession(t *testing.T) {
	u := newUser(t)
	_, laptop := loginAs(t, u.Email, "rahasia123", chromeWindows)
	if status, _ := call(t, http.MethodGet, "/api/profile", u.Token, nil); status != http.StatusUnauthorized {
		t.Fatalf("token perangkat pertama masih berlaku setelah login di perangkat lain (status %d)", status)
	}

	res, out := rawRequest(t, http.MethodGet, "/api/wallets", u.Token, nil, nil)
	if res.StatusCode != http.StatusUnauthorized || out["code"] != "session_replaced" {
		t.Fatalf("respons sesi lama %d %v, ingin 401 session_replaced", res.StatusCode, out)
	}

	status, r := call(t, http.MethodGet, "/api/profile", laptop, nil)
	expectStatus(t, status, http.StatusOK, r)
	if p := decode[struct {
		Device string `json:"last_login_device"`
	}](t, r); p.Device != "Chrome di Windows" {
		t.Fatalf("perangkat terakhir %q", p.Device)
	}

	status, r = call(t, http.MethodPut, "/api/profile/password", laptop, map[string]any{"old_password": "rahasia123", "new_password": "rahasia456"})
	expectStatus(t, status, http.StatusOK, r)
	if _, out := rawRequest(t, http.MethodGet, "/api/wallets", laptop, nil, nil); out["code"] != "password_changed" {
		t.Fatalf("token setelah ganti password %v, ingin password_changed", out)
	}
}

func TestLoginLockoutAndRateLimit(t *testing.T) {
	limiter.SetEnabled(true)
	defer limiter.SetEnabled(false)

	u := newUser(t)
	for i := 0; i < 5; i++ {
		if status, _ := loginAs(t, u.Email, "salah-password", "test"); status != http.StatusUnauthorized {
			t.Fatalf("percobaan %d status %d", i+1, status)
		}
	}
	raw, _ := json.Marshal(map[string]string{"email": strings.ToUpper(u.Email), "password": "rahasia123"})
	res, out := rawRequest(t, http.MethodPost, "/api/auth/login", "", bytes.NewReader(raw), map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" {
		t.Fatalf("setelah 5 gagal: %d %v, ingin 429 dengan Retry-After", res.StatusCode, out)
	}

	other := newUser(t)
	if status, _ := loginAs(t, other.Email, "rahasia123", "test"); status != http.StatusOK {
		t.Fatalf("akun lain ikut terkunci (status %d)", status)
	}

	statuses := map[int]int{}
	for i := 0; i < 12; i++ {
		raw, _ := json.Marshal(map[string]string{"name": "Spam", "email": "spam" + string(rune('a'+i)) + "@contoh.id", "password": "rahasia123"})
		res, _ := rawRequest(t, http.MethodPost, "/api/auth/register", "", bytes.NewReader(raw), map[string]string{
			"Content-Type": "application/json", "X-Forwarded-For": "203.0.113.7",
		})
		statuses[res.StatusCode]++
	}
	if statuses[http.StatusTooManyRequests] != 2 {
		t.Fatalf("pendaftaran dari satu IP %v, ingin 10 diterima lalu 2 ditahan", statuses)
	}
	raw, _ = json.Marshal(map[string]string{"name": "Lain", "email": "lain-ip@contoh.id", "password": "rahasia123"})
	if res, _ := rawRequest(t, http.MethodPost, "/api/auth/register", "", bytes.NewReader(raw), map[string]string{
		"Content-Type": "application/json", "X-Forwarded-For": "203.0.113.8",
	}); res.StatusCode != http.StatusCreated {
		t.Fatalf("IP lain ikut tertahan (status %d)", res.StatusCode)
	}
}

func avatarRequest(t *testing.T, token, filename, contentType string, data []byte) (*http.Response, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="avatar"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)
	part, _ := w.CreatePart(h)
	part.Write(data)
	w.Close()
	return rawRequest(t, http.MethodPost, "/api/profile/avatar", token, &body, map[string]string{"Content-Type": w.FormDataContentType()})
}

func TestAvatarContentIsChecked(t *testing.T) {
	u := newUser(t)
	res, _ := avatarRequest(t, u.Token, "foto.png", "image/png", []byte("<html><script>alert(1)</script></html>"))
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("HTML berlabel image/png diterima (status %d)", res.StatusCode)
	}

	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	res, out := avatarRequest(t, u.Token, "foto.jpg", "application/octet-stream", img.Bytes())
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PNG asli ditolak: %d %v", res.StatusCode, out)
	}
	url, _ := out["data"].(map[string]any)["avatar_url"].(string)
	if !strings.HasSuffix(url, ".png") || strings.Contains(url, u.ID) {
		t.Fatalf("nama file avatar %q, ingin acak dengan ekstensi dari isi file", url)
	}
	defer os.Remove("." + url)

	got, err := http.Get(server.URL + url)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	if got.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(got.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("header file upload %v", got.Header)
	}
}

func TestSecurityHeadersAndBodyLimit(t *testing.T) {
	u := newUser(t)
	res, _ := rawRequest(t, http.MethodGet, "/api/profile", u.Token, nil, nil)
	for k, want := range map[string]string{"X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Cache-Control": "no-store"} {
		if got := res.Header.Get(k); got != want {
			t.Errorf("header %s = %q, ingin %q", k, got, want)
		}
	}

	big := `{"note":"` + strings.Repeat("a", 2<<20) + `"}`
	res, _ = rawRequest(t, http.MethodPost, "/api/transactions", u.Token, strings.NewReader(big), map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("body 2 MB status %d, ingin 413", res.StatusCode)
	}
}
