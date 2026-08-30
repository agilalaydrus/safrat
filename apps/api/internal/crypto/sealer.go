// Package crypto encrypts the identity numbers this system is obliged to keep
// but has no business being able to read from a stolen disk.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	aead        cipher.AEAD
	fingerprint string
	// blindKey is derived from the encryption key rather than being the key
	// itself, so a leaked blinding token gives no purchase on the cipher. One
	// secret to look after, two uses that never share material.
	blindKey []byte
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
	// Derived from the key itself, so the same key always yields the same
	// fingerprint on any machine and in any deployment.
	sum := sha256.Sum256(key)
	// Derived, not the key itself: a blinding token that leaked would otherwise
	// be material from the encryption key. One secret to look after, two uses
	// that share nothing.
	blindKey := sha256.Sum256(append([]byte("safrat-blind-index-v1"), key...))

	return &Sealer{
		aead:        aead,
		fingerprint: hex.EncodeToString(sum[:4]),
		blindKey:    blindKey[:],
	}, nil
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

// Fingerprint identifies a key without revealing it.
//
// Eight hex characters of a SHA-256 over the key. Safe to print in a log, a
// startup banner or a note in a password manager: reversing it would mean
// breaking SHA-256, and 32 random bytes have far more entropy than a
// fingerprint could leak.
//
// It exists so two questions can be answered without handling the secret
// itself: "is the key I am about to deploy the same one that encrypted this
// data" and "which key sealed this record". Comparing fingerprints answers
// both. Without it, a wrong key is discovered one unreadable record at a time,
// long after the deployment that caused it.
func (s *Sealer) Fingerprint() string {
	if s == nil {
		return ""
	}
	return s.fingerprint
}

// Rotator re-seals values from one key to another.
//
// Rotation is two keys held at once, briefly: the old one to read what exists,
// the new one to write it back. Nothing else in this package holds two, and
// nothing should — a type that could silently fall back to a second key would
// make "which key opened this" unanswerable, which is the question the whole
// fingerprint mechanism exists to answer.
type Rotator struct {
	from *Sealer
	to   *Sealer
}

// NewRotator prepares a re-seal from one key to another.
//
// Refuses to rotate a key onto itself. That is not a rotation, and permitting
// it would let somebody believe they had rotated when they had done nothing —
// the fingerprints would agree, the records would be unchanged, and the old key
// they were about to destroy would still be the only one that works.
func NewRotator(fromKey, toKey string) (*Rotator, error) {
	from, err := NewSealer(fromKey)
	if err != nil {
		return nil, fmt.Errorf("kunci lama: %w", err)
	}
	to, err := NewSealer(toKey)
	if err != nil {
		return nil, fmt.Errorf("kunci baru: %w", err)
	}
	if from == nil || to == nil {
		return nil, errors.New("rotasi memerlukan kunci lama dan kunci baru")
	}
	if from.Fingerprint() == to.Fingerprint() {
		return nil, errors.New("kunci lama dan baru sama; tidak ada yang dirotasi")
	}
	return &Rotator{from: from, to: to}, nil
}

// FromFingerprint and ToFingerprint identify the two keys without revealing
// them, for logging progress and for stamping the re-sealed rows.
func (r *Rotator) FromFingerprint() string { return r.from.Fingerprint() }
func (r *Rotator) ToFingerprint() string   { return r.to.Fingerprint() }

// Reseal opens a value with the old key and seals it with the new one.
//
// An empty value stays empty. A value that will not open under the old key is
// an error rather than a skip: rotating past it would leave a row stamped with
// the new key that only the old one can read, and the stamp is what everything
// else trusts.
func (r *Rotator) Reseal(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plaintext, err := r.from.Open(stored)
	if err != nil {
		return "", err
	}
	return r.to.Seal(plaintext)
}

// Blind produces a deterministic, non-reversible token for a value that has to
// stay searchable after it is encrypted.
//
// Sealing uses a random nonce, so the same passport number encrypts differently
// every time — which is exactly right for secrecy and useless for "find the
// pilgrim with this passport". This gives the lookup back: equal inputs produce
// equal tokens, and the token reveals nothing without the key.
//
// HMAC rather than a plain hash. A passport number has far too little entropy
// to survive a dictionary attack on a bare SHA-256, so an attacker holding a
// stolen database could recover every number by hashing candidates. Keyed, that
// attack needs the key — and the key is not in the database.
//
// The value is normalised first: passport numbers are quoted with stray spaces
// and in either case, and a token that differs by whitespace would fail to find
// a record that is plainly there.
func (s *Sealer) Blind(value string) (string, error) {
	normalised := strings.ToUpper(strings.Join(strings.Fields(value), ""))
	if normalised == "" {
		return "", nil
	}
	if s == nil {
		return "", ErrNoKey
	}
	mac := hmac.New(sha256.New, s.blindKey)
	mac.Write([]byte(normalised))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
