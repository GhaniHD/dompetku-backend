// Package integration_test menjalankan server Dompetku sungguhan (wiring Uber FX
// lengkap) terhadap database PostgreSQL khusus test.
//
// Database yang ditunjuk TEST_DB_NAME akan DIKOSONGKAN setiap kali test jalan,
// jadi namanya wajib mengandung kata "test" dan host-nya tidak boleh Supabase.
package integration_test

import (
	"dompetku/api"
	"dompetku/internal/repository"
	"dompetku/internal/service"
	"dompetku/pkg/config"
	"dompetku/pkg/db"
	"dompetku/pkg/ratelimit"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var (
	server       *httptest.Server
	gdb          *gorm.DB
	fakeClaude   *fakeClaudeServer
	recurringSvc service.RecurringService
	activitySvc  service.Activity
	debtSvc      service.DebtService
	limiter      *ratelimit.Limiter
	router       *gin.Engine
)

func TestMain(m *testing.M) {
	if os.Getenv("TEST_DB_NAME") == "" {
		fmt.Println("TEST_DB_NAME tidak diset, integration test dilewati")
		os.Exit(0)
	}

	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup integration test gagal:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run(m *testing.M) (int, error) {
	name := os.Getenv("TEST_DB_NAME")
	host := envOr("TEST_DB_HOST", "localhost")

	if !strings.Contains(strings.ToLower(name), "test") || strings.Contains(strings.ToLower(host), "supabase") {
		return 0, fmt.Errorf("ditolak: TEST_DB_NAME harus mengandung 'test' dan host bukan Supabase, karena database akan dikosongkan")
	}

	fakeClaude = newFakeClaudeServer()
	defer fakeClaude.Close()

	env := map[string]string{
		"DB_HOST":        host,
		"DB_PORT":        envOr("TEST_DB_PORT", "5432"),
		"DB_USER":        envOr("TEST_DB_USER", "postgres"),
		"DB_PASSWORD":    os.Getenv("TEST_DB_PASSWORD"),
		"DB_NAME":        name,
		"DB_SSLMODE":     envOr("TEST_DB_SSLMODE", "disable"),
		"JWT_SECRET":     "integration-test-secret-0123456789abcdef",
		"CLAUDE_API_KEY": "test-key",
		"CLAUDE_API_URL": fakeClaude.URL,
		"ADMIN_EMAILS":   "admin@test.dompetku",
	}

	for k, v := range env {
		os.Setenv(k, v)
	}
	gin.SetMode(gin.TestMode)

	app := fx.New(
		fx.NopLogger,
		config.Module,
		db.Module,
		repository.Module,
		service.Module,
		api.Module,
		fx.Populate(&router, &gdb, &recurringSvc, &debtSvc, &limiter, &activitySvc),
	)
	if err := app.Err(); err != nil {
		return 0, err
	}

	if err := resetSchema(gdb); err != nil {
		return 0, err
	}

	limiter.SetEnabled(false)
	server = httptest.NewServer(router)
	defer server.Close()

	return m.Run(), nil
}

// resetSchema meniru urutan di production: AutoMigrate saat server start,
// lalu file migrations/*.up.sql dijalankan berurutan.
func resetSchema(gdb *gorm.DB) error {
	for _, stmt := range []string{"DROP SCHEMA IF EXISTS public CASCADE", "CREATE SCHEMA public"} {
		if err := gdb.Exec(stmt).Error; err != nil {
			return err
		}
	}

	if err := db.AutoMigrate(gdb); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := gdb.Exec(string(sql)).Error; err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}
