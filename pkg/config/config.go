package config

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"go.uber.org/fx"
)

var Module = fx.Options(fx.Provide(NewAppConf))

type AppConf struct {
	DBHost       string
	DBPort       string
	DBUser       string
	DBPass       string
	DBName       string
	DBSSLMode    string
	JWTSecret    string
	ServerPort   string
	ClaudeAPIKey string
	ClaudeModel  string
	ClaudeAPIURL string
	AdminEmails  []string
	CORSOrigins  []string
	// TrustedProxies: proxy yang header X-Forwarded-For-nya dipercaya untuk
	// menentukan IP asli (Caddy di VPS, load balancer Render).
	TrustedProxies []string

	// Email lewat SMTP (Brevo atau penyedia lain). Kosongkan SMTP_HOST untuk
	// mematikan semua email; verifikasi email ikut nonaktif.
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	MailFrom    string
	MailReplyTo string
	// AppURL: alamat frontend untuk link di email.
	AppURL string
	// APIURL: alamat publik backend, dipakai untuk tombol berhenti langganan satu klik.
	APIURL string
	// BrevoWebhookSecret: dicocokkan dengan ?secret= pada webhook Brevo.
	BrevoWebhookSecret string
	// AdminWhatsApp: nomor WA admin (format 62...) untuk tombol kabari admin setelah daftar.
	AdminWhatsApp string
	// EmailDomainCheck: cek MX record domain email saat daftar.
	EmailDomainCheck bool
}

const minJWTSecretLength = 32

func NewAppConf() (*AppConf, error) {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("[config] .env not found, using OS env")
	}

	cfg := &AppConf{
		DBHost:         getEnv("DB_HOST", "db"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPass:         getEnv("DB_PASSWORD", "postgres"),
		DBName:         getEnv("DB_NAME", "dompetku"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		ServerPort:     getEnv("PORT", getEnv("SERVER_PORT", "8090")),
		ClaudeAPIKey:   os.Getenv("CLAUDE_API_KEY"),
		ClaudeModel:    getEnv("CLAUDE_MODEL", "claude-sonnet-4-20250514"),
		ClaudeAPIURL:   getEnv("CLAUDE_API_URL", "https://api.anthropic.com/v1/messages"),
		AdminEmails:    splitList(os.Getenv("ADMIN_EMAILS"), true),
		CORSOrigins:    splitList(getEnv("CORS_ORIGINS", "http://localhost:5173,https://myfulus.my.id,https://www.myfulus.my.id,https://dompetku-new.vercel.app"), false),
		TrustedProxies: splitList(getEnv("TRUSTED_PROXIES", "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"), false),

		SMTPHost:           os.Getenv("SMTP_HOST"),
		SMTPPort:           getEnv("SMTP_PORT", "587"),
		SMTPUser:           os.Getenv("SMTP_USER"),
		SMTPPass:           os.Getenv("SMTP_PASS"),
		MailFrom:           getEnv("MAIL_FROM", "MyFulus <noreply@myfulus.my.id>"),
		MailReplyTo:        os.Getenv("MAIL_REPLY_TO"),
		AppURL:             strings.TrimRight(getEnv("APP_URL", "https://myfulus.my.id"), "/"),
		APIURL:             strings.TrimRight(os.Getenv("API_URL"), "/"),
		AdminWhatsApp:      normalizePhone(os.Getenv("ADMIN_WHATSAPP")),
		BrevoWebhookSecret: os.Getenv("BREVO_WEBHOOK_SECRET"),
		EmailDomainCheck:   getEnv("EMAIL_DOMAIN_CHECK", "true") != "false",
	}

	if len(cfg.JWTSecret) < minJWTSecretLength {
		return nil, errors.New("[config] JWT_SECRET wajib diisi, minimal 32 karakter")
	}

	return cfg, nil
}

// normalizePhone menyisakan digit dan mengubah awalan 0 menjadi 62.
func normalizePhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	n := b.String()
	if strings.HasPrefix(n, "0") {
		n = "62" + n[1:]
	}
	if len(n) < 9 {
		return ""
	}
	return n
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func splitList(raw string, lower bool) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if lower {
			part = strings.ToLower(part)
		}
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (c *AppConf) IsAdminEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, e := range c.AdminEmails {
		if e == email {
			return true
		}
	}
	return false
}
