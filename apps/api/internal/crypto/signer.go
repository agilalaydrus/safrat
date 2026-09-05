package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrNoSigningKey is returned rather than silently exporting unsigned data —
// see AuditExportService (C4, TUGAS-PANEL-SAAS.md). An auditor export nobody
// can verify the integrity of is worse than no export at all.
var ErrNoSigningKey = errors.New("kunci penandatangan ekspor auditor belum dikonfigurasi (AUDIT_EXPORT_SIGNING_KEY)")

var errBadSigningKey = errors.New("AUDIT_EXPORT_SIGNING_KEY harus 32 byte terenkode base64")

// Signer HMAC-signs audit export manifests.
//
// Deliberately not Sealer: this never decrypts anything, only ever signs with
// whatever key is configured right now. Rotating this key needs no overlap
// window the way KYC_ENCRYPTION_KEY does — there is nothing already on disk
// that was signed with the old key and still needs verifying by this
// process. Each export's manifest simply records which key signed it (via
// Fingerprint), so a rotation is just "the next export after the restart
// carries a different fingerprint," nothing more.
type Signer struct {
	key []byte
	// fingerprint is precomputed at construction — same reasoning as
	// Sealer.fingerprint, and the same derivation, so a fingerprint means the
	// same thing everywhere in this codebase.
	fingerprint string
}

// NewSigner builds a signer from a base64-encoded 32-byte key.
//
// An empty key returns a nil Signer and no error, matching NewSealer: a
// deployment without one starts and serves everything that does not export
// audit logs. What it must not do is produce an export claiming to be signed
// when it is not, which is why Sign on a nil Signer fails instead of
// returning an empty signature.
func NewSigner(base64Key string) (*Signer, error) {
	trimmed := strings.TrimSpace(base64Key)
	if trimmed == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil || len(key) != 32 {
		return nil, errBadSigningKey
	}
	sum := sha256.Sum256(key)
	return &Signer{key: key, fingerprint: hex.EncodeToString(sum[:4])}, nil
}

// Fingerprint identifies the key without revealing it — same derivation as
// Sealer.Fingerprint.
func (s *Signer) Fingerprint() string {
	if s == nil {
		return ""
	}
	return s.fingerprint
}

// Sign returns hex-encoded HMAC-SHA256(data, key).
func (s *Signer) Sign(data []byte) (string, error) {
	if s == nil {
		return "", ErrNoSigningKey
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}
