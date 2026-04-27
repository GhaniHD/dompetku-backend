package db

import (
	"dompetku/pkg/config"
	"fmt"
	"log"

	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

var Module = fx.Options(fx.Provide(ConnectDB))

func ConnectDB(conf *config.AppConf) (*gorm.DB, error) {
	// Implementation for connecting to the database
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Jakarta",
		conf.DBHost,
		conf.DBUser,
		conf.DBPass,
		conf.DBName,
		conf.DBPort,
		conf.DBSSLMode,
	)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	log.Println("successfully connected to the database")

	return db, nil
}
