package service

import "go.uber.org/fx"

var Module = fx.Module("service",
	fx.Provide(NewAuthService),
	fx.Provide(NewNotificationService),
	fx.Provide(NewTransactionService),
	fx.Provide(NewCategoryService),
    fx.Provide(NewProfileService),
	fx.Provide(NewWalletService),
	fx.Provide(NewBudgetService),
	fx.Provide(NewReportService),
	fx.Provide(NewAnalysisService),
)