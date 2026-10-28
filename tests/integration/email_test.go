package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type emailStatus struct {
	MailEnabled  bool   `json:"mail_enabled"`
	Verified     bool   `json:"verified"`
	Required     bool   `json:"required"`
	PendingEmail string `json:"pending_email"`
	Bounced      bool   `json:"bounced"`
	Prefs        struct {
		Weekly     bool `json:"weekly"`
		LoginAlert bool `json:"login_alert"`
		Reminders  bool `json:"reminders"`
	} `json:"prefs"`
}

type challenge struct {
	VerifyToken string `json:"verify_token"`
	Email       string `json:"email"`
	ResendIn    int    `json:"resend_in"`
}

func emailProfile(t *testing.T, token string) (string, emailStatus) {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/profile", token, nil)
	expectStatus(t, status, http.StatusOK, r)
	p := decode[struct {
		Email       string      `json:"email"`
		EmailStatus emailStatus `json:"email_status"`
	}](t, r)
	return p.Email, p.EmailStatus
}

func loginChallenge(t *testing.T, email string) challenge {
	t.Helper()
	status, r := call(t, http.MethodPost, "/api/auth/login", "", map[string]any{"email": email, "password": "rahasia123"})
	expectStatus(t, status, http.StatusForbidden, r)
	if r.Code != "email_unverified" {
		t.Fatalf("kode error %q, ingin email_unverified", r.Code)
	}
	ch := decode[challenge](t, r)
	if ch.VerifyToken == "" || !strings.Contains(ch.Email, "***") {
		t.Fatalf("challenge tidak lengkap: %+v", ch)
	}
	return ch
}

func verifyLogin(t *testing.T, token, code string) (int, apiResponse) {
	t.Helper()
	return call(t, http.MethodPost, "/api/auth/verify", "", map[string]any{"verify_token": token, "code": code})
}

// verifiedUser membuat akun aktif yang emailnya sudah terverifikasi.
func verifiedUser(t *testing.T) testUser {
	t.Helper()
	u := newUser(t)
	gdb.Exec("UPDATE users SET email_verified_at = now() WHERE id = ?", u.ID)
	return u
}

func TestEmailDisabledKeepsOldFlow(t *testing.T) {
	u := newUser(t)
	_, st := emailProfile(t, u.Token)
	if st.MailEnabled || st.Required {
		t.Fatalf("tanpa SMTP verifikasi tidak boleh diwajibkan: %+v", st)
	}
	status, r := call(t, http.MethodGet, "/api/auth/features", "", nil)
	expectStatus(t, status, http.StatusOK, r)
	if decode[struct {
		Email bool `json:"email"`
	}](t, r).Email {
		t.Fatal("fitur email harus mati tanpa SMTP")
	}
	status, r = call(t, http.MethodPost, "/api/auth/forgot-password", "", map[string]any{"email": u.Email})
	expectStatus(t, status, http.StatusServiceUnavailable, r)
}

func TestInviteRegisterNeedsCode(t *testing.T) {
	withMail(t)
	admin := adminUser(t)
	inv := decode[struct {
		Code string `json:"code"`
	}](t, adminCall(t, admin, http.MethodPost, "/invites", map[string]any{}, http.StatusCreated))

	email := fmt.Sprintf("undang-%s@test.dompetku", uuid.NewString()[:8])
	status, r := call(t, http.MethodPost, "/api/auth/register", "", map[string]any{"name": "Undangan", "email": email, "password": "rahasia123", "invite_code": inv.Code})
	expectStatus(t, status, http.StatusCreated, r)
	reg := decode[struct {
		Token                string `json:"token"`
		VerificationRequired bool   `json:"verification_required"`
		VerifyToken          string `json:"verify_token"`
	}](t, r)
	if reg.Token != "" || !reg.VerificationRequired || reg.VerifyToken == "" {
		t.Fatalf("daftar lewat undangan harus minta kode dulu: %+v", reg)
	}
	code := lastCode(t, email)

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	status, r = verifyLogin(t, reg.VerifyToken, wrong)
	expectStatus(t, status, http.StatusBadRequest, r)
	if !strings.Contains(r.Message, "sisa 4") {
		t.Fatalf("pesan kode salah %q", r.Message)
	}
	status, r = verifyLogin(t, "palsu."+reg.VerifyToken, code)
	expectStatus(t, status, http.StatusBadRequest, r)

	status, r = verifyLogin(t, reg.VerifyToken, code[:3]+" "+code[3:])
	expectStatus(t, status, http.StatusOK, r)
	if r.Token == "" {
		t.Fatal("verifikasi berhasil harus mengembalikan token")
	}
	session := r.Token
	_, st := emailProfile(t, session)
	if !st.Verified || st.Required {
		t.Fatalf("status email setelah verifikasi %+v", st)
	}

	status, r = verifyLogin(t, reg.VerifyToken, code)
	expectStatus(t, status, http.StatusBadRequest, r)

	// Jeda kirim ulang berlaku per alamat: ganti email tepat setelah verifikasi tetap mengirim kode.
	newEmail := strings.Replace(email, "undang-", "pindah-", 1)
	status, r = call(t, http.MethodPost, "/api/profile/email", session, map[string]any{"email": newEmail, "password": "rahasia123"})
	expectStatus(t, status, http.StatusOK, r)
	lastCode(t, newEmail)
}

func TestApprovedUserVerifiesAtFirstLogin(t *testing.T) {
	withMail(t)
	admin := adminUser(t)
	u := registerPending(t, "Baru")
	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/approve", nil, http.StatusOK)

	mails := flushMail(t, u.Email)
	if len(mails) != 1 || !strings.Contains(mails[0].Text, "disetujui") {
		t.Fatalf("email persetujuan tidak terkirim: %d email", len(mails))
	}
	code := codeRe.FindString(mails[0].Text)
	if code == "" {
		t.Fatal("email persetujuan harus memuat kode")
	}

	ch := loginChallenge(t, u.Email)
	if ch.ResendIn <= 0 {
		t.Fatalf("kode baru saja dikirim, resend_in harus > 0: %d", ch.ResendIn)
	}
	status, r := verifyLogin(t, ch.VerifyToken, code)
	expectStatus(t, status, http.StatusOK, r)

	login(t, u.Email, "rahasia123")
}

func TestExistingUserGracePeriodAndResendLimits(t *testing.T) {
	u := newUser(t)
	withMail(t)

	token := login(t, u.Email, "rahasia123")
	_, st := emailProfile(t, token)
	if !st.Required || st.Verified {
		t.Fatalf("akun lama harus diminta verifikasi: %+v", st)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM users WHERE id = ? AND email_verify_due_at > now() + interval '6 days'", u.ID); n != 1 {
		t.Fatal("akun lama harus mendapat masa tenggang 7 hari")
	}

	gdb.Exec("UPDATE users SET email_verify_due_at = now() - interval '1 minute' WHERE id = ?", u.ID)
	ch := loginChallenge(t, u.Email)
	first := lastCode(t, u.Email)

	status, r := call(t, http.MethodPost, "/api/auth/verify/send", "", map[string]any{"verify_token": ch.VerifyToken})
	expectStatus(t, status, http.StatusTooManyRequests, r)
	if r.Code != "code_cooldown" {
		t.Fatalf("kode error %q", r.Code)
	}

	allowResend(t, u.ID)
	status, r = call(t, http.MethodPost, "/api/auth/verify/send", "", map[string]any{"verify_token": ch.VerifyToken})
	expectStatus(t, status, http.StatusOK, r)
	second := lastCode(t, u.Email)
	if first != second {
		status, r = verifyLogin(t, ch.VerifyToken, first)
		expectStatus(t, status, http.StatusBadRequest, r)
	}

	for i := 0; i < 3; i++ {
		allowResend(t, u.ID)
		status, r = call(t, http.MethodPost, "/api/auth/verify/send", "", map[string]any{"verify_token": ch.VerifyToken})
		expectStatus(t, status, http.StatusOK, r)
	}
	allowResend(t, u.ID)
	gdb.Exec("UPDATE email_codes SET created_at = now() - interval '10 minutes' WHERE user_id = ?", u.ID)
	status, r = call(t, http.MethodPost, "/api/auth/verify/send", "", map[string]any{"verify_token": ch.VerifyToken})
	expectStatus(t, status, http.StatusTooManyRequests, r)
	if !strings.Contains(r.Message, "batas kirim kode") {
		t.Fatalf("batas per jam tidak berlaku: %q", r.Message)
	}

	code := lastCode(t, u.Email)
	status, r = verifyLogin(t, ch.VerifyToken, code)
	expectStatus(t, status, http.StatusOK, r)
}

func TestCodeLocksAfterFiveWrongAttempts(t *testing.T) {
	withMail(t)
	u := newUser(t)
	gdb.Exec("UPDATE users SET email_verify_due_at = now() - interval '1 minute' WHERE id = ?", u.ID)
	ch := loginChallenge(t, u.Email)
	code := lastCode(t, u.Email)
	wrong := fmt.Sprintf("%06d", (mustAtoi(code)+1)%1000000)
	for i := 0; i < 5; i++ {
		status, r := verifyLogin(t, ch.VerifyToken, wrong)
		expectStatus(t, status, http.StatusBadRequest, r)
	}
	status, r := verifyLogin(t, ch.VerifyToken, code)
	expectStatus(t, status, http.StatusBadRequest, r)
	if !strings.Contains(r.Message, "terlalu banyak") {
		t.Fatalf("kode harus terkunci, pesan %q", r.Message)
	}
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func TestChangeEmailNeedsPasswordAndCode(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)
	newEmail := fmt.Sprintf("baru-%s@test.dompetku", uuid.NewString()[:8])

	status, r := call(t, http.MethodPut, "/api/profile", u.Token, map[string]any{"name": "Tester", "email": newEmail})
	expectStatus(t, status, http.StatusBadRequest, r)
	status, r = call(t, http.MethodPut, "/api/profile", u.Token, map[string]any{"name": "Nama Baru"})
	expectStatus(t, status, http.StatusOK, r)

	status, r = call(t, http.MethodPost, "/api/profile/email", u.Token, map[string]any{"email": newEmail, "password": "salah-sekali"})
	expectStatus(t, status, http.StatusForbidden, r)
	status, r = call(t, http.MethodPost, "/api/profile/email", u.Token, map[string]any{"email": "admin@test.dompetku", "password": "rahasia123"})
	expectStatus(t, status, http.StatusBadRequest, r)
	status, r = call(t, http.MethodPost, "/api/profile/email", u.Token, map[string]any{"email": "orang@gmial.com", "password": "rahasia123"})
	expectStatus(t, status, http.StatusBadRequest, r)
	if r.Code != "email_address" || !strings.Contains(r.Message, "gmail.com") {
		t.Fatalf("saran salah ketik tidak muncul: %q %q", r.Code, r.Message)
	}
	status, r = call(t, http.MethodPost, "/api/profile/email", u.Token, map[string]any{"email": "a@mailinator.com", "password": "rahasia123"})
	expectStatus(t, status, http.StatusBadRequest, r)

	status, r = call(t, http.MethodPost, "/api/profile/email", u.Token, map[string]any{"email": strings.ToUpper(newEmail), "password": "rahasia123"})
	expectStatus(t, status, http.StatusOK, r)
	current, st := emailProfile(t, u.Token)
	if current != u.Email || st.PendingEmail != newEmail {
		t.Fatalf("email lama harus tetap aktif sampai diverifikasi: %s, pending %s", current, st.PendingEmail)
	}
	code := lastCode(t, newEmail)

	status, r = call(t, http.MethodPost, "/api/profile/email/verify", u.Token, map[string]any{"code": code})
	expectStatus(t, status, http.StatusOK, r)
	current, st = emailProfile(t, u.Token)
	if current != newEmail || st.PendingEmail != "" || !st.Verified {
		t.Fatalf("email belum berganti: %s %+v", current, st)
	}
	notice := flushMail(t, u.Email)
	if len(notice) != 1 || !strings.Contains(notice[0].Text, "diganti") {
		t.Fatalf("email lama harus menerima pemberitahuan, dapat %d", len(notice))
	}
	login(t, newEmail, "rahasia123")
	if n := countRows(t, "SELECT COUNT(*) FROM auth_events WHERE user_id = ? AND event = 'email_changed'", u.ID); n != 1 {
		t.Fatalf("event email_changed %d", n)
	}
}

func TestCancelPendingEmail(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)
	newEmail := fmt.Sprintf("batal-%s@test.dompetku", uuid.NewString()[:8])
	status, r := call(t, http.MethodPost, "/api/profile/email", u.Token, map[string]any{"email": newEmail, "password": "rahasia123"})
	expectStatus(t, status, http.StatusOK, r)
	code := lastCode(t, newEmail)
	status, r = call(t, http.MethodDelete, "/api/profile/email/pending", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	status, r = call(t, http.MethodPost, "/api/profile/email/verify", u.Token, map[string]any{"code": code})
	expectStatus(t, status, http.StatusConflict, r)
	current, _ := emailProfile(t, u.Token)
	if current != u.Email {
		t.Fatalf("email berubah setelah dibatalkan: %s", current)
	}
}

func TestForgotPassword(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)

	status, r := call(t, http.MethodPost, "/api/auth/forgot-password", "", map[string]any{"email": "tidak-ada@test.dompetku"})
	expectStatus(t, status, http.StatusOK, r)
	unknownMsg := r.Message
	status, r = call(t, http.MethodPost, "/api/auth/forgot-password", "", map[string]any{"email": strings.ToUpper(u.Email)})
	expectStatus(t, status, http.StatusOK, r)
	if r.Message != unknownMsg {
		t.Fatal("jawaban lupa password tidak boleh membedakan email terdaftar")
	}
	mails := flushMail(t, u.Email)
	if len(mails) != 1 {
		t.Fatalf("email atur ulang %d", len(mails))
	}
	i := strings.Index(mails[0].Text, "token=")
	if i < 0 || !strings.Contains(mails[0].Text, "https://app.test/atur-ulang-password") {
		t.Fatalf("link atur ulang tidak ada: %s", mails[0].Text)
	}
	token := strings.Fields(mails[0].Text[i+len("token="):])[0]
	if n := countRows(t, "SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = ? AND expires_at < now() + interval '31 minutes' AND expires_at > now() + interval '29 minutes'", u.ID); n != 1 {
		t.Fatal("link harus berlaku 30 menit")
	}
	status, r = call(t, http.MethodPost, "/api/auth/reset-password", "", map[string]any{"token": token, "password": "rahasiaBaru1"})
	expectStatus(t, status, http.StatusOK, r)
	status, r = call(t, http.MethodPost, "/api/auth/reset-password", "", map[string]any{"token": token, "password": "rahasiaBaru2"})
	expectStatus(t, status, http.StatusBadRequest, r)
	login(t, u.Email, "rahasiaBaru1")
}

func loginWithAgent(t *testing.T, email, agent string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"email": email, "password": "rahasia123"})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", agent)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status %d", res.StatusCode)
	}
}

const (
	alertChrome  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
	safariIPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
)

func TestLoginAlertOnNewDevice(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)
	loginWithAgent(t, u.Email, alertChrome)
	flushMail(t, u.Email)
	loginWithAgent(t, u.Email, alertChrome)
	if n := len(flushMail(t, u.Email)); n != 0 {
		t.Fatalf("perangkat sama tidak perlu peringatan, dapat %d", n)
	}
	loginWithAgent(t, u.Email, safariIPhone)
	mails := flushMail(t, u.Email)
	if len(mails) != 1 || !strings.Contains(mails[0].Text, "perangkat") {
		t.Fatalf("peringatan login %d", len(mails))
	}
	if !strings.HasPrefix(mails[0].Unsubscribe, "https://api.test/api/email/unsubscribe?token=") {
		t.Fatalf("header berhenti langganan %q", mails[0].Unsubscribe)
	}
}

func unsubToken(t *testing.T, header string) string {
	t.Helper()
	parsed, err := url.Parse(header)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Query().Get("token")
}

// runScheduledWeek menjalankan email terjadwal pukul 08.00 WIB selama 7 hari
// berturut-turut, sehingga setiap pengguna pasti kebagian hari ringkasannya.
func runScheduledWeek(t *testing.T) {
	t.Helper()
	wib := time.FixedZone("WIB", 7*3600)
	now := time.Now().In(wib)
	base := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, wib)
	for d := 0; d < 7; d++ {
		emailSvc.RunScheduled(base.AddDate(0, 0, d))
	}
}

func TestWeeklySummaryAndUnsubscribe(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)
	wallet := createWallet(t, u, "Tunai", 1_000_000)
	food := createCategory(t, u, "Makan", "expense")
	yesterday := time.Now().In(time.FixedZone("WIB", 7*3600)).AddDate(0, 0, -1).Format("2006-01-02") + "T09:00:00+07:00"
	body := txBody(wallet, food, 125_000, "expense")
	body["date"] = yesterday
	status, r := call(t, http.MethodPost, "/api/transactions", u.Token, body)
	expectStatus(t, status, http.StatusCreated, r)

	runScheduledWeek(t)
	mails := flushMail(t, u.Email)
	if len(mails) != 1 {
		t.Fatalf("ringkasan mingguan %d email, ingin 1", len(mails))
	}
	m := mails[0]
	if strings.Contains(m.Subject, "Rp") || !strings.Contains(m.Text, "Rp 125.000") || !strings.Contains(m.Text, "Makan") {
		t.Fatalf("isi ringkasan salah:\nsubjek %q\n%s", m.Subject, m.Text)
	}
	runScheduledWeek(t)
	if n := len(flushMail(t, u.Email)); n != 0 {
		t.Fatalf("ringkasan terkirim ganda: %d", n)
	}

	token := unsubToken(t, m.Unsubscribe)
	status, r = call(t, http.MethodPost, "/api/email/unsubscribe?token="+url.QueryEscape(token[:len(token)-2]+"xx"), "", nil)
	expectStatus(t, status, http.StatusBadRequest, r)
	status, r = call(t, http.MethodPost, "/api/email/unsubscribe", "", map[string]any{"token": token})
	expectStatus(t, status, http.StatusOK, r)
	_, st := emailProfile(t, u.Token)
	if st.Prefs.Weekly || !st.Prefs.LoginAlert || !st.Prefs.Reminders {
		t.Fatalf("hanya ringkasan mingguan yang berhenti: %+v", st.Prefs)
	}

	gdb.Exec("UPDATE users SET weekly_sent_on = NULL WHERE id = ?", u.ID)
	runScheduledWeek(t)
	if n := len(flushMail(t, u.Email)); n != 0 {
		t.Fatalf("ringkasan tetap terkirim setelah berhenti langganan: %d", n)
	}

	status, r = call(t, http.MethodPut, "/api/profile/email-prefs", u.Token, map[string]any{"weekly": true, "login_alert": false, "reminders": true})
	expectStatus(t, status, http.StatusOK, r)
	_, st = emailProfile(t, u.Token)
	if !st.Prefs.Weekly || st.Prefs.LoginAlert {
		t.Fatalf("preferensi tidak tersimpan: %+v", st.Prefs)
	}
}

func TestDueReminderTheDayBefore(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)
	wib := time.FixedZone("WIB", 7*3600)
	now := time.Now().In(wib)
	today := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, wib)
	tomorrow := today.AddDate(0, 0, 1).Format("2006-01-02")
	gdb.Exec("INSERT INTO debts (user_id, kind, counterparty, principal, due_date) VALUES (?, 'payable', 'Budi', 500000, ?)", u.ID, tomorrow)
	gdb.Exec("INSERT INTO debts (user_id, kind, counterparty, principal, paid_amount, due_date) VALUES (?, 'payable', 'Lunas', 100000, 100000, ?)", u.ID, tomorrow)

	emailSvc.RunScheduled(today.Add(-2 * time.Hour))
	if n := len(flushMail(t, u.Email)); n != 0 {
		t.Fatalf("pengingat tidak boleh dikirim sebelum 07.00 WIB: %d", n)
	}
	emailSvc.RunScheduled(today)
	mails := flushMail(t, u.Email)
	var reminders []string
	for _, m := range mails {
		if strings.Contains(m.Subject, "Pengingat") {
			reminders = append(reminders, m.Text)
		}
	}
	if len(reminders) != 1 || !strings.Contains(reminders[0], "Budi") || strings.Contains(reminders[0], "Lunas") {
		t.Fatalf("pengingat salah: %v", reminders)
	}
	emailSvc.RunScheduled(today.Add(time.Hour))
	for _, m := range flushMail(t, u.Email) {
		if strings.Contains(m.Subject, "Pengingat") {
			t.Fatal("pengingat terkirim dua kali di hari yang sama")
		}
	}
}

func TestBrevoBounceStopsMail(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)

	status, r := call(t, http.MethodPost, "/api/email/webhook/brevo?secret=salah", "", map[string]any{"event": "hard_bounce", "email": u.Email})
	expectStatus(t, status, http.StatusNotFound, r)
	status, r = call(t, http.MethodPost, "/api/email/webhook/brevo?secret=hook-secret-123", "", []map[string]any{{"event": "hard_bounce", "email": strings.ToUpper(u.Email)}})
	expectStatus(t, status, http.StatusOK, r)
	_, st := emailProfile(t, u.Token)
	if !st.Bounced {
		t.Fatal("profil harus menandai email terpental")
	}

	loginWithAgent(t, u.Email, alertChrome)
	loginWithAgent(t, u.Email, safariIPhone)
	if n := len(flushMail(t, u.Email)); n != 0 {
		t.Fatalf("email biasa tetap terkirim ke alamat terpental: %d", n)
	}

	status, r = call(t, http.MethodPost, "/api/auth/forgot-password", "", map[string]any{"email": u.Email})
	expectStatus(t, status, http.StatusOK, r)
	if n := len(flushMail(t, u.Email)); n != 1 {
		t.Fatalf("email penting tetap dicoba, dapat %d", n)
	}
}

func TestOutboxRetriesThenFails(t *testing.T) {
	withMail(t)
	u := verifiedUser(t)
	fakeMail.mu.Lock()
	fakeMail.fail = true
	fakeMail.mu.Unlock()
	call(t, http.MethodPost, "/api/auth/forgot-password", "", map[string]any{"email": u.Email})

	at := time.Now()
	for i := 1; i <= 5; i++ {
		if _, err := outboxSvc.Flush(at); err != nil {
			t.Fatal(err)
		}
		var row struct {
			Status   string
			Attempts int
		}
		gdb.Raw("SELECT status, attempts FROM email_outbox WHERE user_id = ?", u.ID).Scan(&row)
		if row.Attempts != i {
			t.Fatalf("percobaan ke-%d tercatat %d", i, row.Attempts)
		}
		if i < 5 && row.Status != "pending" || i == 5 && row.Status != "failed" {
			t.Fatalf("status setelah %d percobaan: %s", i, row.Status)
		}
		at = at.Add(3 * time.Hour)
	}
}

func TestAdminMarksEmailVerifiedAndSeesStats(t *testing.T) {
	withMail(t)
	admin := adminUser(t)
	u := newUser(t)

	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/verify-email", nil, http.StatusBadRequest)
	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/verify-email", map[string]any{"password": "salah"}, http.StatusForbidden)
	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/verify-email", adminPassword, http.StatusOK)
	adminCall(t, admin, http.MethodPost, "/users/"+u.ID+"/verify-email", adminPassword, http.StatusConflict)
	_, st := emailProfile(t, u.Token)
	if !st.Verified {
		t.Fatal("email harus terverifikasi setelah ditandai admin")
	}

	users := decode[[]struct {
		ID            string `json:"id"`
		EmailVerified bool   `json:"email_verified"`
	}](t, adminCall(t, admin, http.MethodGet, "/users?q="+url.QueryEscape(u.Email), nil, http.StatusOK))
	if len(users) != 1 || !users[0].EmailVerified {
		t.Fatalf("daftar pengguna admin %+v", users)
	}
	logs := decode[[]struct {
		Action string `json:"action"`
	}](t, adminCall(t, admin, http.MethodGet, "/logs", nil, http.StatusOK))
	if len(logs) == 0 || logs[0].Action != "verify_email" {
		t.Fatalf("log admin %v", logs)
	}

	call(t, http.MethodPost, "/api/auth/forgot-password", "", map[string]any{"email": u.Email})
	flushMail(t, u.Email)
	r := adminCall(t, admin, http.MethodGet, "/email-stats", nil, http.StatusOK)
	stats := decode[struct {
		Enabled   bool  `json:"enabled"`
		SentToday int64 `json:"sent_today"`
		DailyCap  int   `json:"daily_cap"`
		Verified  int64 `json:"verified"`
	}](t, r)
	if !stats.Enabled || stats.SentToday < 1 || stats.DailyCap != 250 || stats.Verified < 1 {
		t.Fatalf("statistik email %+v", stats)
	}
	if strings.Contains(string(r.Data), u.Email) {
		t.Fatal("statistik tidak boleh memuat alamat email")
	}

	status, rr := call(t, http.MethodGet, "/api/admin/email-stats", u.Token, nil)
	expectStatus(t, status, http.StatusForbidden, rr)
}

func TestNewSignupNotifiesAdmin(t *testing.T) {
	withMail(t)
	adminUser(t)
	u := registerPending(t, "Calon Pengguna")
	mails := flushMail(t, testAdminEmail)
	if len(mails) != 1 || !strings.Contains(mails[0].Text, "Calon Pengguna") || !strings.Contains(mails[0].Text, "/admin/persetujuan") {
		t.Fatalf("email pendaftar baru ke admin: %d", len(mails))
	}
	if strings.Contains(mails[0].Text, u.Email) {
		t.Fatal("email pendaftar di notifikasi admin harus disamarkan")
	}

	status, r := call(t, http.MethodGet, "/api/auth/features", "", nil)
	expectStatus(t, status, http.StatusOK, r)
	if got := decode[struct {
		AdminWhatsApp string `json:"admin_whatsapp"`
	}](t, r).AdminWhatsApp; got != "6281234567890" {
		t.Fatalf("nomor WA admin %q", got)
	}
}
