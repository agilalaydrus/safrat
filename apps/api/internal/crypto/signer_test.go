package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

func newTestSigner(t *testing.T) *Signer {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := NewSigner(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer
}

func TestSignerUnconfiguredIsNilNotError(t *testing.T) {
	signer, err := NewSigner("")
	if err != nil || signer != nil {
		t.Fatalf("kunci kosong seharusnya nil,nil, dapat %v,%v", signer, err)
	}
	if _, err := signer.Sign([]byte("data")); !errors.Is(err, ErrNoSigningKey) {
		t.Fatalf("Sign pada signer nil seharusnya ErrNoSigningKey, dapat %v", err)
	}
	if signer.Fingerprint() != "" {
		t.Fatalf("Fingerprint pada signer nil seharusnya kosong")
	}
}

func TestSignerRejectsBadKeyLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := NewSigner(short); err == nil {
		t.Fatalf("kunci pendek seharusnya ditolak")
	}
}

// TestSignerDeterministicAndTamperEvident is the property the whole feature
// depends on: the same data always produces the same signature (an auditor
// re-hashing the same CSV must get the same answer), and any change to the
// data — even one byte — produces a different one (the whole point of
// signing the export in the first place).
func TestSignerDeterministicAndTamperEvident(t *testing.T) {
	signer := newTestSigner(t)
	original, err := signer.Sign([]byte("sha256-of-the-csv"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	again, err := signer.Sign([]byte("sha256-of-the-csv"))
	if err != nil || again != original {
		t.Fatalf("tanda tangan yang sama seharusnya konsisten: %q vs %q", original, again)
	}
	tampered, err := signer.Sign([]byte("sha256-of-the-csv-tampered"))
	if err != nil || tampered == original {
		t.Fatalf("data yang berubah seharusnya menghasilkan tanda tangan berbeda")
	}
}

func TestSignerFingerprintIdentifiesKeyNotContent(t *testing.T) {
	signer := newTestSigner(t)
	fp1 := signer.Fingerprint()
	fp2 := signer.Fingerprint()
	if fp1 == "" || fp1 != fp2 {
		t.Fatalf("fingerprint seharusnya stabil untuk kunci yang sama, dapat %q dan %q", fp1, fp2)
	}
	other := newTestSigner(t)
	if other.Fingerprint() == fp1 {
		t.Fatalf("dua kunci acak seharusnya tidak pernah punya fingerprint sama")
	}
}
