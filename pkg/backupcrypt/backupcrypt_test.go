package backupcrypt

import (
	"bytes"
	"errors"
	"testing"
)

func TestSealOpen(t *testing.T) {
	plain := []byte(`{"format":"dompetku-backup","version":1}`)
	sealed, err := Seal(plain, "rahasia-backup")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("dompetku-backup")) {
		t.Fatal("isi backup terbaca tanpa password")
	}

	got, err := Open(sealed, "rahasia-backup")
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("buka backup: %v %q", err, got)
	}
	if _, err := Open(sealed, "password-lain"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("password salah: %v", err)
	}

	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	if _, err := Open(tampered, "rahasia-backup"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("file diubah: %v", err)
	}
	header := append([]byte(nil), sealed...)
	header[6] ^= 1
	if _, err := Open(header, "rahasia-backup"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("salt diubah: %v", err)
	}
	if _, err := Open([]byte("bukan backup sama sekali, cuma teks biasa"), "x"); !errors.Is(err, ErrNotBackup) {
		t.Fatalf("file lain: %v", err)
	}

	again, _ := Seal(plain, "rahasia-backup")
	if bytes.Equal(again, sealed) {
		t.Fatal("dua backup identik, salt/nonce tidak acak")
	}
}
