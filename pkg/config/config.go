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
}

const minJWTSecretLength = 32

func NewAppConf() (*AppConf, error) {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("[config] .env not found, using OS env")
	}

	cfg := &AppConf{
		DBHost:       getEnv("DB_HOST", "db"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       getEnv("DB_USER", "postgres"),
		DBPass:       getEnv("DB_PASSWORD", "postgres"),
		DBName:       getEnv("DB_NAME", "dompetku"),
		DBSSLMode:    getEnv("DB_SSLMODE", "disable"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		ServerPort:   getEnv("PORT", getEnv("SERVER_PORT", "8090")),
		ClaudeAPIKey: os.Getenv("CLAUDE_API_KEY"),
		ClaudeModel:  getEnv("CLAUDE_MODEL", "claude-sonnet-4-20250514"),
		ClaudeAPIURL: getEnv("CLAUDE_API_URL", "https://api.anthropic.com/v1/messages"),
		AdminEmails:  splitList(os.Getenv("ADMIN_EMAILS"), true),
		CORSOrigins:  splitList(getEnv("CORS_ORIGINS", "http://localhost:5173,https://dompetku-new.vercel.app"), false),
	}

	if len(cfg.JWTSecret) < minJWTSecretLength {
		return nil, errors.New("[config] JWT_SECRET wajib diisi, minimal 32 karakter")
	}

	return cfg, nil
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
