package repository

import (
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/crypto"
	"github.com/jackc/pgx/v5/pgtype"
)

// kycSealer encrypts the identity numbers this system must keep but has no
// business being able to read from a stolen disk or backup.
//
// A package-level value, which is normally a smell and is justified here: it is
// one key for the whole process, set once at startup and never changed.
// Threading it through every conversion function — toAgent, toPilgrim and the
// rest are pure and take no dependencies — would touch dozens of call sites for
// no benefit, and every one of them a chance to forget.
var kycSealer *crypto.Sealer

// SetKYCSealer installs the key. Called once from main before serving.
func SetKYCSealer(sealer *crypto.Sealer) { kycSealer = sealer }

// KYCKeyFingerprint identifies the installed key without revealing it, for
// startup logging and for stamping onto the records it seals.
func KYCKeyFingerprint() string { return kycSealer.Fingerprint() }

// sealKYC encrypts a value for storage.
//
// Returns an error rather than falling back, so a deployment missing its key
// refuses to store an identity number instead of quietly writing it in the
// clear — which would look like success and stay invisible until a breach.
func sealKYC(plaintext string) (string, error) {
	return kycSealer.Seal(plaintext)
}

// openKYC decrypts a stored value for a caller that cannot return an error.
//
// A value that will not open — a key rotated without re-encrypting, a corrupted
// row — yields empty rather than the ciphertext. Showing base64 in a field
// labelled NIK would be worse than showing nothing, and would invite somebody
// to "fix" it by overwriting the record. The failure is reported so it is not
// merely absent.
func openKYC(stored string) string {
	plaintext, err := kycSealer.Open(stored)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("open KYC field: %w", err))
		return ""
	}
	return plaintext
}

// pgInt8Ptr turns an optional amount into a nullable column value, keeping
// "not applicable" distinct from zero. A product with no face value is not a
// product whose face value is nothing.
func pgInt8Ptr(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

// sealPassport encrypts a passport number and produces the token that keeps it
// findable.
//
// Both or neither: a row carrying ciphertext without a blind token cannot be
// looked up, and one carrying a token without ciphertext claims a secret it does
// not hold. The schema enforces the pairing; this is what makes the pairing
// true at the point of writing.
//
// An empty passport stays empty rather than becoming an encrypted empty string,
// so "not recorded" and "recorded as blank" remain different things.
func sealPassport(plaintext string) (sealed, blind, fingerprint string, err error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", "", "", nil
	}
	sealed, err = kycSealer.Seal(plaintext)
	if err != nil {
		return "", "", "", err
	}
	blind, err = kycSealer.Blind(plaintext)
	if err != nil {
		return "", "", "", err
	}
	return sealed, blind, kycSealer.Fingerprint(), nil
}

// blindPassport builds the lookup token for a search.
//
// Separate from sealPassport because a search has nothing to encrypt: it turns
// what somebody typed into the token that will match the stored row, and the
// normalisation inside Blind is what makes "c1234567" find "C1234567".
func blindPassport(query string) (string, error) {
	return kycSealer.Blind(query)
}
