package db

import (
	"dompetku/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate dipakai oleh server dan integration test. Tabel transfers tidak
// termasuk karena dibuat lewat migrations/002_create_transfers.up.sql.
func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&model.User{},
		&model.Category{},
		&model.Transaction{},
		&model.Wallet{},
		&model.Budget{},
		&model.Notification{},
	)
}
