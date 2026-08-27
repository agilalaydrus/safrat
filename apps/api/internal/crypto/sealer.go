// Package crypto encrypts the identity numbers this system is obliged to keep
// but has no business being able to read from a stolen disk.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// prefix marks a value this package produced. Its presence is how an already
// encrypted value is told from a legacy plaintext one, which matters because a
// live database has both during the migration and neither can be corrupted.
const prefix = "v1."

var (
	// ErrNoKey is returned rather than falling back to plaintext. Silently
	// storing an unencrypted identity number because a variable was missing is
	// exactly the failure this package exists to prevent, and it would be
	// invisible until a breach made it obvious.
	ErrNoKey = errors.New("kunci enkripsi KYC belum dikonfigurasi (KYC_ENCRYPTION_KEY)")

	errBadKey  = errors.New("kunci enkripsi KYC harus 32 byte terenkode base64")
	errCorrupt = errors.New("nilai terenkripsi rusak atau kuncinya berbeda")
)

// Sealer encrypts and decrypts identity fields.
//
// AES-256-GCM with a random nonce per value, so the same identity number
// encrypts differently every time and equal ciphertexts reveal nothing. That
// rules out searching by these fields, which is acceptable because nothing does
// — they are written and read back for one record at a time.
//
// GCM also authenticates: a tampered ciphertext fails to open rather than
// decrypting to something plausible.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer builds a sealer from a base64-encoded 32-byte key.
//
// An empty key returns a nil Sealer and no error, so a deployment without one
// starts and serves everything that does not touch KYC. What it must not do is
// accept KYC writes, which is why Seal on a nil Sealer fails rather than
// passing the value through.
func NewSealer(base64Key string) (*Sealer, error) {
	trimmed := strings.TrimSpace(base64Key)
	if trimmed == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil || len(key) != 32 {
		return nil, errBadKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal encrypts a value. An empty value stays empty — an absent identity number
// is not a secret, and encrypting nothing would make "not provided" and
// "provided" indistinguishable in the database.
func (s *Sealer) Seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if s == nil {
		return "", ErrNoKey
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := s.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Open decrypts a value.
//
// A value without the prefix is returned unchanged. That is what lets a live
// database hold both encrypted and not-yet-migrated rows without either being
// mangled — and it is deliberately one-directional: an unencrypted value can be
// read, but nothing new is ever written unencrypted.
func (s *Sealer) Open(stored string) (string, error) {
	if stored == "" || !strings.HasPrefix(stored, prefix) {
		return stored, nil
	}
	if s == nil {
		return "", ErrNoKey
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", fmt.Errorf("%w: %v", errCorrupt, err)
	}
	nonceSize := s.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", errCorrupt
	}
	plaintext, err := s.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		// Either the ciphertext was altered or the key is not the one it was
		// sealed with. Both must fail loudly: returning something on a key
		// mismatch would quietly corrupt every record it touched.
		return "", fmt.Errorf("%w: %v", errCorrupt, err)
	}
	return string(plaintext), nil
}

// IsSealed reports whether a stored value is already encrypted, for the
// migration that walks existing rows.
func IsSealed(stored string) bool {
	return strings.HasPrefix(stored, prefix)
}
