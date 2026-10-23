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
)
