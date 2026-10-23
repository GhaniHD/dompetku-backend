package service

import (
	"dompetku/internal/repository"
	"errors"
)

var (
	ErrTransactionNotFound      = errors.New("transaksi tidak ditemukan")
	ErrWalletNotFound           = repository.ErrWalletNotFound
	ErrCategoryNotFound         = errors.New("kategori tidak ditemukan")
	ErrCategoryTypeMismatch     = errors.New("tipe kategori tidak sesuai dengan tipe transaksi")
	ErrBudgetCategoryNotExpense = errors.New("anggaran hanya bisa dibuat untuk kategori pengeluaran")

	ErrTransferNotFound         = errors.New("transfer tidak ditemukan")
	ErrTransferSameWallet       = errors.New("dompet asal dan tujuan tidak boleh sama")
	ErrInsufficientBalance      = errors.New("saldo dompet tidak cukup; hanya kartu kredit yang boleh bersaldo minus")
	ErrTransactionIsTransferFee = errors.New("transaksi ini adalah biaya admin transfer; ubah atau hapus lewat transfernya")

	ErrWalletArchived         = errors.New("dompet sudah diarsipkan; aktifkan kembali sebelum dipakai")
	ErrWalletNotEmpty         = errors.New("saldo dompet harus 0 sebelum diarsipkan; pindahkan saldonya lewat transfer atau penyesuaian")
	ErrNegativeOpeningBalance = errors.New("saldo awal hanya boleh minus untuk kartu kredit")
	ErrWalletTypeNegative     = errors.New("dompet bersaldo minus hanya boleh bertipe kartu kredit; sesuaikan saldonya dulu")
	ErrAdjustmentNoChange     = errors.New("saldo aktual sama dengan saldo tercatat, tidak ada yang disesuaikan")

	ErrNotificationNotFound = errors.New("notifikasi tidak ditemukan")
)
