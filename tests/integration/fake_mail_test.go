package integration_test

import (
	"dompetku/internal/service"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeMailer menggantikan SMTP. Default mati sehingga test lama berjalan
// seperti saat SMTP belum dikonfigurasi.
type fakeMailer struct {
	mu      sync.Mutex
	enabled bool
	fail    bool
	sent    []service.Mail
}

func (f *fakeMailer) Enabled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enabled
}

func (f *fakeMailer) Send(m service.Mail) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("smtp mati")
	}
	f.sent = append(f.sent, m)
	return nil
}

// withMail menyalakan email selama satu test dan mengosongkan antrean.
func withMail(t *testing.T) {
	t.Helper()
	gdb.Exec("DELETE FROM email_outbox")
	fakeMail.mu.Lock()
	fakeMail.enabled, fakeMail.fail, fakeMail.sent = true, false, nil
	fakeMail.mu.Unlock()
	t.Cleanup(func() {
		fakeMail.mu.Lock()
		fakeMail.enabled, fakeMail.fail, fakeMail.sent = false, false, nil
		fakeMail.mu.Unlock()
		gdb.Exec("DELETE FROM email_outbox")
	})
}

// flushMail mengirim antrean lalu mengambil email baru yang terkirim ke alamat itu.
func flushMail(t *testing.T, to string) []service.Mail {
	t.Helper()
	if _, err := outboxSvc.Flush(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("flush: %v", err)
	}
	fakeMail.mu.Lock()
	defer fakeMail.mu.Unlock()
	var out, rest []service.Mail
	for _, m := range fakeMail.sent {
		if strings.EqualFold(m.To, to) {
			out = append(out, m)
		} else {
			rest = append(rest, m)
		}
	}
	fakeMail.sent = rest
	return out
}

var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

// lastCode mengambil kode OTP dari email terakhir ke alamat itu.
func lastCode(t *testing.T, to string) string {
	t.Helper()
	mails := flushMail(t, to)
	for i := len(mails) - 1; i >= 0; i-- {
		if m := codeRe.FindStringSubmatch(mails[i].Subject); m != nil {
			return m[1]
		}
	}
	t.Fatalf("tidak ada email kode untuk %s", to)
	return ""
}

// allowResend memundurkan waktu kode terakhir agar jeda 60 detik terlewati.
func allowResend(t *testing.T, userID string) {
	t.Helper()
	gdb.Exec("UPDATE email_codes SET created_at = created_at - interval '2 minutes' WHERE user_id = ?", userID)
}
