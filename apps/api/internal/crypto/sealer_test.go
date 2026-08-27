package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func newTestSealer(t *testing.T) *Sealer {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sealer, err := NewSealer(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("new sealer: %v", err)
	}
	return sealer
}

func TestSealAndOpenRoundTrip(t *testing.T) {
	sealer := newTestSealer(t)
	// Short values are round-tripped but not searched for inside the
	// ciphertext: a one- or two-character string turns up in random base64
	// often enough to fail on its own, which is a flaky test rather than a
	// finding. Real identity numbers are long, and those are checked.
	for _, plaintext := range []string{"3174012345670001", "09.254.294.1-407.000", "x", ""} {
		sealed, err := sealer.Seal(plaintext)
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		if len(plaintext) >= 8 && strings.Contains(sealed, plaintext) {
			t.Fatalf("the stored value still contains the plaintext: %s", sealed)
		}
		opened, err := sealer.Open(sealed)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if opened != plaintext {
			t.Fatalf("round trip gave %q, want %q", opened, plaintext)
		}
	}
}

// The property that actually matters, checked the way it should be: the
// ciphertext decodes to bytes that are nothing like the input.
func TestSealedBytesDoNotContainThePlaintext(t *testing.T) {
	sealer := newTestSealer(t)
	const nik = "3174012345670001"
	sealed, err := sealer.Seal(nik)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, "v1."))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(string(raw), nik) {
		t.Fatal("the ciphertext bytes contain the identity number")
	}
}

// The same identity number must not encrypt to the same bytes twice, or equal
// ciphertexts would reveal which records share an identity.
func TestSealIsNotDeterministic(t *testing.T) {
	sealer := newTestSealer(t)
	first, _ := sealer.Seal("3174012345670001")
	second, _ := sealer.Seal("3174012345670001")
	if first == second {
		t.Fatal("the same value sealed to identical bytes twice")
	}
	// Both still open to the same thing.
	openedFirst, _ := sealer.Open(first)
	openedSecond, _ := sealer.Open(second)
	if openedFirst != openedSecond || openedFirst != "3174012345670001" {
		t.Fatalf("opened to %q and %q", openedFirst, openedSecond)
	}
}

// A live database holds both encrypted and not-yet-migrated rows. Reading must
// cope with both without mangling either.
func TestOpenPassesThroughLegacyPlaintext(t *testing.T) {
	sealer := newTestSealer(t)
	opened, err := sealer.Open("3174012345670001")
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	if opened != "3174012345670001" {
		t.Fatalf("legacy value became %q", opened)
	}
	if IsSealed("3174012345670001") {
		t.Fatal("a plaintext value was reported as sealed")
	}
	sealed, _ := sealer.Seal("3174012345670001")
	if !IsSealed(sealed) {
		t.Fatal("a sealed value was not reported as sealed")
	}
}

// Tampering must fail, not decrypt to something plausible.
func TestOpenRefusesTamperedOrForeignCiphertext(t *testing.T) {
	sealer := newTestSealer(t)
	sealed, _ := sealer.Seal("3174012345670001")

	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := sealer.Open(tampered); err == nil {
		t.Fatal("a tampered value opened")
	}

	// A different key must not silently return rubbish — that would corrupt
	// every record it touched, invisibly.
	other := newTestSealer(t)
	if _, err := other.Open(sealed); err == nil {
		t.Fatal("a value sealed with another key opened")
	}
}

// Falling back to plaintext because a variable is missing is the exact failure
// this package exists to prevent, and it would stay invisible until a breach.
func TestWithoutAKeySealingFailsRatherThanStoringPlaintext(t *testing.T) {
	sealer, err := NewSealer("")
	if err != nil {
		t.Fatalf("empty key should not be an error: %v", err)
	}
	if sealer != nil {
		t.Fatal("an empty key produced a sealer")
	}
	if _, err := sealer.Seal("3174012345670001"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("sealing without a key returned %v, want ErrNoKey", err)
	}
	// Reading legacy plaintext still works, so a deployment without a key is
	// degraded rather than broken.
	if opened, err := sealer.Open("3174012345670001"); err != nil || opened != "3174012345670001" {
		t.Fatalf("legacy read without a key gave %q (%v)", opened, err)
	}
	// But it cannot read anything real.
	if _, err := sealer.Open("v1.abc"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("opening sealed data without a key returned %v", err)
	}
}

func TestNewSealerRejectsAnUnusableKey(t *testing.T) {
	for _, key := range []string{"not-base64!!", base64.StdEncoding.EncodeToString([]byte("too short"))} {
		if _, err := NewSealer(key); err == nil {
			t.Errorf("accepted an unusable key: %q", key)
		}
	}
}
