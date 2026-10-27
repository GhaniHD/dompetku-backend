// Package backupcrypt mengenkripsi file backup DompetKu dengan password.
//
// Format: "DPKB1" | salt (16) | nonce (12) | AES-256-GCM(gzip(isi)).
// Kunci diturunkan dengan Argon2id; tag GCM sekaligus mendeteksi file yang diubah.
package backupcrypt

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

var magic = []byte("DPKB1")

const (
	saltSize  = 16
	nonceSize = 12
	// MaxPlainSize membatasi ukuran isi setelah didekompres (anti zip bomb).
	MaxPlainSize = 200 << 20
)

var (
	ErrNotBackup = errors.New("file bukan backup DompetKu")
	ErrDecrypt   = errors.New("password backup salah atau file rusak")
	ErrTooLarge  = errors.New("isi backup terlalu besar")
)

func key(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 2, 32*1024, 2, 32)
}

func Seal(plain []byte, password string) ([]byte, error) {
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(plain); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	salt := make([]byte, saltSize)
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key(password, salt))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(magic)+saltSize+nonceSize+zipped.Len()+gcm.Overhead())
	out = append(out, magic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	// Header ikut diautentikasi agar tidak bisa ditukar.
	return gcm.Seal(out, nonce, zipped.Bytes(), out[:len(magic)+saltSize+nonceSize]), nil
}

func Open(data []byte, password string) ([]byte, error) {
	head := len(magic) + saltSize + nonceSize
	if len(data) < head+16 || !bytes.Equal(data[:len(magic)], magic) {
		return nil, ErrNotBackup
	}
	salt := data[len(magic) : len(magic)+saltSize]
	nonce := data[len(magic)+saltSize : head]

	block, err := aes.NewCipher(key(password, salt))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	zipped, err := gcm.Open(nil, nonce, data[head:], data[:head])
	if err != nil {
		return nil, ErrDecrypt
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, ErrDecrypt
	}
	plain, err := io.ReadAll(io.LimitReader(zr, MaxPlainSize+1))
	if err != nil {
		return nil, ErrDecrypt
	}
	if len(plain) > MaxPlainSize {
		return nil, ErrTooLarge
	}
	return plain, nil
}
