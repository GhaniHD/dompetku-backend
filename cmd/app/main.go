package main

import (
	"context"
	"dompetku/api"
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

func MigrateDB(gdb *gorm.DB) error {
	log.Println("Menjalankan migrasi database...")

	if err := db.AutoMigrate(gdb); err != nil {
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
		fx.Invoke(func(gdb *gorm.DB, cfg *config.AppConf) {
			if err := service.EnsureAdmins(gdb, cfg.AdminEmails); err != nil {
				log.Printf("[admin] gagal menetapkan admin dari ADMIN_EMAILS: %v", err)
			}
		}),
		fx.Invoke(service.StartScheduler),
		fx.Invoke(service.StartActivityCleanup),
		fx.Invoke(service.StartMailWorker),
		fx.Invoke(StartServer),
	)

	app.Run()
}
