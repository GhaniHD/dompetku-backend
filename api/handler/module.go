package handler

import "go.uber.org/fx"

var Module = fx.Module("handler",
	fx.Provide(NewAuthHandler),
	fx.Provide(NewNotificationHandler),
	fx.Provide(NewTransactionHandler),
	fx.Provide(NewCategoryHandler),
	fx.Provide(NewProfileHandler),
	fx.Provide(NewWalletHandler),
	fx.Provide(NewBudgetHandler),
	fx.Provide(NewReportHandler),
	fx.Provide(NewAnalysisHandler),
)