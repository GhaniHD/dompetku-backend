package api

import (
	"dompetku/api/handler"
	"dompetku/internal/middleware"
	"dompetku/internal/repository"
	"dompetku/internal/service"
	claudepkg "dompetku/pkg/claude"
	"dompetku/pkg/config"
	"dompetku/pkg/ratelimit"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

var Module = fx.Options(
	handler.Module,
	fx.Provide(NewClaudeClient),
	fx.Provide(ratelimit.New),
	fx.Provide(NewRouter),
)

func NewClaudeClient(cfg *config.AppConf) *claudepkg.Client {
	return claudepkg.NewClient(cfg.ClaudeAPIKey, cfg.ClaudeModel, cfg.ClaudeAPIURL)
}

func NewRouter(
	authHandler *handler.AuthHandler,
	transactionHandler *handler.TransactionHandler,
	categoryHandler *handler.CategoryHandler,
	profileHandler *handler.ProfileHandler,
	walletHandler *handler.WalletHandler,
	budgetHandler *handler.BudgetHandler,
	notificationHandler *handler.NotificationHandler,
	reportHandler *handler.ReportHandler,
	analysisHandler *handler.AnalysisHandler,
	transferHandler *handler.TransferHandler,
	exportHandler *handler.ExportHandler,
	recurringHandler *handler.RecurringHandler,
	debtGoalHandler *handler.DebtGoalHandler,
	adminHandler *handler.AdminHandler,
	backupHandler *handler.BackupHandler,
	userRepo repository.UserRepository,
	activity service.Activity,
	cfg *config.AppConf,
	limiter *ratelimit.Limiter,
) *gin.Engine {
	r := gin.Default()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Printf("[router] TRUSTED_PROXIES tidak valid: %v", err)
	}

	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORSMiddleware(cfg))
	r.Use(middleware.BodyLimit(1<<20, map[string]int64{
		"/avatar":         3 << 20,
		"/backup/preview": handler.MaxBackupFileSize + 1<<20,
		"/backup/restore": handler.MaxBackupFileSize + 1<<20,
	}))
	r.Use(middleware.Gzip())
	r.Static("/uploads", "./uploads")

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	auth := r.Group("/api/auth")
	{
		auth.POST("/register", middleware.RateLimitIP(limiter, "register", 10, time.Hour), authHandler.Register)
		auth.POST("/login", middleware.RateLimitIP(limiter, "login", 30, 15*time.Minute), authHandler.Login)
		auth.GET("/invite/:code", middleware.RateLimitIP(limiter, "invite", 30, 15*time.Minute), authHandler.CheckInvite)
		auth.POST("/reset-password", middleware.RateLimitIP(limiter, "reset", 10, 15*time.Minute), authHandler.ResetPassword)
	}

	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware(cfg, userRepo), middleware.TrackPresence(activity), middleware.RateLimitUser(limiter, 1200, time.Minute))
	{
		protected.POST("/auth/logout", authHandler.Logout)
		profileHandler.RegisterRoutes(protected)
		notificationHandler.RegisterRoutes(protected)
		adminHandler.RegisterPublic(protected)
		adminHandler.RegisterRoutes(protected.Group("/admin", middleware.RequireAdmin()))

		// Fitur keuangan: akun admin ditolak di semua endpoint ini.
		finance := protected.Group("", middleware.ForbidAdmin())
		transactionHandler.RegisterRoutes(finance)
		categoryHandler.RegisterRoutes(finance)
		budgetHandler.RegisterRoutes(finance)
		walletHandler.RegisterRoutes(finance)
		reportHandler.RegisterRoutes(finance)
		analysisHandler.RegisterRoutes(finance)
		transferHandler.RegisterRoutes(finance)
		exportHandler.RegisterRoutes(finance)
		recurringHandler.RegisterRoutes(finance)
		debtGoalHandler.RegisterRoutes(finance)

		backup := finance.Group("/backup")
		backup.POST("/export", middleware.RateLimitUserNamed(limiter, "backup-export", 5, time.Hour), backupHandler.Export)
		backup.POST("/preview", middleware.RateLimitUserNamed(limiter, "backup-preview", 10, time.Hour), backupHandler.Preview)
		backup.POST("/restore", middleware.RateLimitUserNamed(limiter, "backup-restore", 5, time.Hour), backupHandler.Restore)
	}

	return r
}
