package repository

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/crypto"
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
