package repository

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Urutan aman terhadap foreign key antar tabel data keuangan.
var userDataTables = []string{
	"budgets", "goals", "debt_movements", "debts", "transfers",
	"balance_adjustments", "transactions", "recurring_transactions", "wallets", "categories",
}

// DeleteUserData menghapus permanen seluruh data keuangan pengguna.
// Akun, profil, dan notifikasi tidak disentuh.
func DeleteUserData(tx *gorm.DB, userID uuid.UUID) error {
	for _, table := range userDataTables {
		if err := tx.Exec("DELETE FROM "+table+" WHERE user_id = ?", userID).Error; err != nil {
			return fmt.Errorf("hapus %s: %w", table, err)
		}
	}
	return nil
}

// PurgeUser menghapus permanen akun beserta semua datanya, sehingga email
// tersebut bisa dipakai mendaftar lagi. Notifikasi dan token reset ikut
// terhapus lewat ON DELETE CASCADE.
func PurgeUser(tx *gorm.DB, userID uuid.UUID) error {
	if err := DeleteUserData(tx, userID); err != nil {
		return err
	}
	for _, table := range []string{"email_codes", "email_outbox"} {
		if err := tx.Exec("DELETE FROM "+table+" WHERE user_id = ?", userID).Error; err != nil {
			return fmt.Errorf("hapus %s: %w", table, err)
		}
	}
	return tx.Exec("DELETE FROM users WHERE id = ?", userID).Error
}
