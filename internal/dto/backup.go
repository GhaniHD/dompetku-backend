package dto

import "time"

type BackupExportRequest struct {
	Password       string `json:"password"        binding:"required"`
	BackupPassword string `json:"backup_password" binding:"required,min=8,max=128"`
}

// BackupPreview diringkas dari isi file sebelum dipulihkan.
type BackupPreview struct {
	CreatedAt    time.Time `json:"created_at"`
	SourceName   string    `json:"source_name"`
	SourceEmail  string    `json:"source_email"`
	Wallets      int       `json:"wallets"`
	Categories   int       `json:"categories"`
	Transactions int       `json:"transactions"`
	Transfers    int       `json:"transfers"`
	Budgets      int       `json:"budgets"`
	Recurring    int       `json:"recurring"`
	Debts        int       `json:"debts"`
	Goals        int       `json:"goals"`
	TotalBalance float64   `json:"total_balance"`
	FirstDate    string    `json:"first_date"`
	LastDate     string    `json:"last_date"`
}
