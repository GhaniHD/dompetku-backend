package config

import (
	"errors"
	"log"
	"os"

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
