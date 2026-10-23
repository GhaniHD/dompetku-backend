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
	ErrInsufficientBalance      = errors.New("saldo dompet asal tidak cukup untuk nominal dan biaya admin")
	ErrTransactionIsTransferFee = errors.New("transaksi ini adalah biaya admin transfer; ubah atau hapus lewat transfernya")
)
