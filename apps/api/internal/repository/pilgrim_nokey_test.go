package repository

import (
	"errors"
	"testing"

	"github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/domain"
)

// Without a key, creating a jamaah now fails. That is the operational cost of
// sealing this field, and it must be a loud refusal rather than a silent
// plaintext write — the whole point is that a dump gives nothing.
func TestCreatingAPilgrimWithoutAKeyIsRefused(t *testing.T) {
	previous := kycSealer
	SetKYCSealer(nil)
	t.Cleanup(func() { SetKYCSealer(previous) })

	_, _, _, err := sealPassport("C1234567")
	if !errors.Is(err, crypto.ErrNoKey) {
		t.Fatalf("tanpa kunci dikembalikan %v, mau ErrNoKey — jangan pernah diam-diam menyimpan teks polos", err)
	}

	// An empty passport is still allowed: "not recorded" needs no key.
	sealed, blind, fp, err := sealPassport("")
	if err != nil || sealed != "" || blind != "" || fp != "" {
		t.Fatalf("paspor kosong seharusnya lewat tanpa kunci: %q/%q/%q (%v)", sealed, blind, fp, err)
	}
	_ = domain.PilgrimInput{}
}
