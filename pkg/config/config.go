package config

import (
	"github.com/joho/godotenv"
	"go.uber.org/fx"
	"log"
	"os"
)

var Module = fx.Options(fx.Provide(NewAppConf))

type AppConf struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPass     string
	DBName     string
	DBSSLMode  string
	JWTSecret  string
	ServerPort string
	ClaudeAPIKey string
}

func NewAppConf() (*AppConf, error) {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("[config] .env not found, using OS env:", err)
	}

	// Debug: print semua env variable
	log.Println("DB_HOST:", os.Getenv("DB_HOST"))
	log.Println("DB_PASSWORD:", os.Getenv("DB_PASSWORD"))
	log.Println("DB_USER:", os.Getenv("DB_USER"))

	return &AppConf{
		DBHost:     getEnv("DB_HOST", "db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPass:     getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "dompetku"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
		JWTSecret:  getEnv("JWT_SECRET", "your-secret-key"),
		ServerPort:   getEnv("PORT", getEnv("SERVER_PORT", "8090")),
		ClaudeAPIKey: os.Getenv("CLAUDE_API_KEY"),
	}, nil
}

// Helper function untuk get env dengan default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}