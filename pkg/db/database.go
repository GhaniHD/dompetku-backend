package db

import (
	"dompetku/pkg/config"
	"fmt"
	"log"
	"time"

	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

var Module = fx.Options(fx.Provide(ConnectDB))

func ConnectDB(conf *config.AppConf) (*gorm.DB, error) {
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

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// Koneksi dipakai ulang agar request paralel tidak membuka koneksi TLS baru ke database.
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	log.Println("successfully connected to the database")

	return db, nil
}
