package repository

import "go.uber.org/fx"

var Module = fx.Module("repository",
	fx.Provide(NewUserRepository),
	fx.Provide(NewNotificationRepository),
	fx.Provide(NewTransactionRepository),
	fx.Provide(NewCategoryRepository),
	fx.Provide(NewProfileRepository),
	fx.Provide(NewWalletRepository),
	fx.Provide(NewBudgetRepository),
	fx.Provide(NewReportRepository),
	fx.Provide(NewAnalysisRepository),
)