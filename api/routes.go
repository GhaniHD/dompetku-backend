package api

import (
	"dompetku/api/handler"
	"dompetku/internal/middleware"
	"dompetku/internal/repository"
	claudepkg "dompetku/pkg/claude"
	"dompetku/pkg/config"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

var Module = fx.Options(
	handler.Module,
	fx.Provide(NewClaudeClient),
	fx.Provide(NewRouter),
)

func NewClaudeClient(cfg *config.AppConf) *claudepkg.Client {
	return claudepkg.NewClient(cfg.ClaudeAPIKey)
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
	userRepo repository.UserRepository,
	cfg *config.AppConf,
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
	protected.Use(middleware.AuthMiddleware(cfg, userRepo))
	{
		profileHandler.RegisterRoutes(protected)
		transactionHandler.RegisterRoutes(protected)
		categoryHandler.RegisterRoutes(protected)
		budgetHandler.RegisterRoutes(protected)
		walletHandler.RegisterRoutes(protected)
		notificationHandler.RegisterRoutes(protected)
		reportHandler.RegisterRoutes(protected)
		analysisHandler.RegisterRoutes(protected)
		transferHandler.RegisterRoutes(protected)
	}

	return r
}
