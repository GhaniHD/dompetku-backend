package api

import (
	"dompetku/api/handler"
	"dompetku/internal/middleware"
	claudepkg "dompetku/pkg/claude"
	"dompetku/pkg/config"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// Module mendaftarkan semua dependency API ke Uber FX
var Module = fx.Options(
	handler.Module,
	fx.Provide(NewClaudeClient), // ← provide Claude client ke FX container
	fx.Provide(NewRouter),
)

// NewClaudeClient membuat instance Claude API client dari config
func NewClaudeClient(cfg *config.AppConf) *claudepkg.Client {
	return claudepkg.NewClient(cfg.ClaudeAPIKey)
}

// NewRouter membuat dan mengkonfigurasi semua route aplikasi
func NewRouter(
	authHandler         *handler.AuthHandler,
	transactionHandler  *handler.TransactionHandler,
	categoryHandler     *handler.CategoryHandler,
	profileHandler      *handler.ProfileHandler,
	walletHandler       *handler.WalletHandler,
	budgetHandler       *handler.BudgetHandler,
	notificationHandler *handler.NotificationHandler,
	reportHandler       *handler.ReportHandler,
	analysisHandler     *handler.AnalysisHandler,
	cfg                 *config.AppConf,
) *gin.Engine {
	r := gin.Default()

	r.Use(middleware.CORSMiddleware())
	r.Static("/uploads", "./uploads")

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	auth := r.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware(cfg))
	{
		profileHandler.RegisterRoutes(protected)
		transactionHandler.RegisterRoutes(protected)
		categoryHandler.RegisterRoutes(protected)
		budgetHandler.RegisterRoutes(protected)
		walletHandler.RegisterRoutes(protected)
		notificationHandler.RegisterRoutes(protected)
		reportHandler.RegisterRoutes(protected)
		analysisHandler.RegisterRoutes(protected)
	}

	return r
}