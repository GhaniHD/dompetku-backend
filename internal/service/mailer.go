package service

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"dompetku/pkg/config"
)

type Mail struct {
	To          string
	Subject     string
	HTML        string
	Text        string
	Unsubscribe string
}

// Mailer mengirim satu email. Enabled false berarti SMTP belum dikonfigurasi:
// tidak ada email yang dikirim dan verifikasi email tidak diwajibkan.
type Mailer interface {
	Enabled() bool
	Send(m Mail) error
}

type disabledMailer struct{}

func (disabledMailer) Enabled() bool   { return false }
func (disabledMailer) Send(Mail) error { return errors.New("SMTP belum dikonfigurasi") }

type smtpMailer struct {
	host, port, user, pass string
	from                   *mail.Address
	replyTo                string
}

func NewMailer(cfg *config.AppConf) Mailer {
	if cfg.SMTPHost == "" {
		return disabledMailer{}
	}
	from, err := mail.ParseAddress(cfg.MailFrom)
	if err != nil {
		from = &mail.Address{Name: "MyFulus", Address: cfg.MailFrom}
	}
	return &smtpMailer{host: cfg.SMTPHost, port: cfg.SMTPPort, user: cfg.SMTPUser, pass: cfg.SMTPPass, from: from, replyTo: cfg.MailReplyTo}
}

func (m *smtpMailer) Enabled() bool { return true }

func (m *smtpMailer) Send(msg Mail) error {
	body, err := buildMessage(m.from, m.replyTo, msg)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(m.host, m.port)
	var auth smtp.Auth
	if m.user != "" {
		auth = smtp.PlainAuth("", m.user, m.pass, m.host)
	}
	if m.port != "465" {
		return smtp.SendMail(addr, auth, m.from.Address, []string{msg.To}, body)
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", addr, &tls.Config{ServerName: m.host})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return err
	}
	defer c.Close()
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(m.from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(msg.To); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func qp(s string) string {
	var buf bytes.Buffer
	w := quotedprintable.NewWriter(&buf)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return buf.String()
}

// buildMessage menyusun email multipart (teks + HTML) dengan header yang
// dibutuhkan penyedia email agar tidak dianggap spam.
func buildMessage(from *mail.Address, replyTo string, m Mail) ([]byte, error) {
	if strings.ContainsAny(m.To, "\r\n") || strings.ContainsAny(m.Subject, "\r\n") {
		return nil, errors.New("header email tidak valid")
	}
	domain := "myfulus.my.id"
	if at := strings.LastIndex(from.Address, "@"); at >= 0 {
		domain = from.Address[at+1:]
	}
	boundary := "mf-" + randomHex(12)
	var b strings.Builder
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", from.String())
	h("To", m.To)
	if replyTo != "" {
		h("Reply-To", replyTo)
	}
	h("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("Message-ID", fmt.Sprintf("<%s@%s>", randomHex(16), domain))
	if m.Unsubscribe != "" {
		h("List-Unsubscribe", "<"+m.Unsubscribe+">")
		h("List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
	}
	h("MIME-Version", "1.0")
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n%s\r\n", boundary, part.kind, qp(part.body))
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String()), nil
}
