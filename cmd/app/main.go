package main

import (
	"context"
	"dompetku/api"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/internal/service"
	"dompetku/pkg/config"
	"dompetku/pkg/db"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

func MigrateDB(db *gorm.DB) error {
	log.Println("Menjalankan migrasi database...")

	if err := db.AutoMigrate(
		&model.User{},
		&model.Category{},
		&model.Transaction{},
		&model.Wallet{},
		&model.Budget{},
		&model.Notification{},
	); err != nil {
		return err
	}

	log.Println("Migrasi database selesai")
	return nil
}

func StartServer(lc fx.Lifecycle, router *gin.Engine, cfg *config.AppConf) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				address := "0.0.0.0:" + cfg.ServerPort
				fmt.Printf("🚀 Server berjalan di http://localhost:%s\n", cfg.ServerPort)
				fmt.Printf("🌐 Listening on %s (accessible from outside)\n", address)

				if err := router.Run(address); err != nil {
					log.Fatal("Server gagal:", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			fmt.Println("🛑 Server sedang berhenti...")
			return nil
		},
	})
}

func main() {
	app := fx.New(
		config.Module,
		db.Module,

		repository.Module,
		service.Module,
		api.Module,

		fx.Invoke(MigrateDB),
		fx.Invoke(StartServer),
	)

	app.Run()
}