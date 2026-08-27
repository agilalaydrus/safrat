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

// A fingerprint answers "is this the same key" without anybody handling the
// key, which is what makes a forgotten or rotated key diagnosable rather than
// discovered one unreadable record at a time.
func TestFingerprintIdentifiesAKeyWithoutRevealingIt(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key)

	first, err := NewSealer(encoded)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	// The same key, loaded again — as it would be on another machine or after a
	// restart — must fingerprint identically, or comparing them proves nothing.
	second, err := NewSealer(encoded)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	if first.Fingerprint() != second.Fingerprint() {
		t.Fatal("the same key produced two different fingerprints")
	}
	if first.Fingerprint() == "" {
		t.Fatal("no fingerprint was produced")
	}

	// A different key must not collide, or a wrong key would look correct.
	other := newTestSealer(t)
	if other.Fingerprint() == first.Fingerprint() {
		t.Fatal("two different keys share a fingerprint")
	}

	// And it must not be the key, nor any part of it.
	if strings.Contains(encoded, first.Fingerprint()) {
		t.Fatal("the fingerprint appears inside the key itself")
	}

	// No key, no fingerprint — and no panic.
	var absent *Sealer
	if absent.Fingerprint() != "" {
		t.Fatal("a missing key produced a fingerprint")
	}
}

// Rotation is the one operation that holds two keys at once. What matters is
// that it never leaves a record stamped with a key that cannot open it.
func TestRotatorResealsBetweenKeys(t *testing.T) {
	oldKey := make([]byte, 32)
	newKey := make([]byte, 32)
	if _, err := rand.Read(oldKey); err != nil {
		t.Fatalf("key: %v", err)
	}
	if _, err := rand.Read(newKey); err != nil {
		t.Fatalf("key: %v", err)
	}
	oldEncoded := base64.StdEncoding.EncodeToString(oldKey)
	newEncoded := base64.StdEncoding.EncodeToString(newKey)

	sealedByOld, err := mustSealer(t, oldEncoded).Seal("3174012345670001")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	rotator, err := NewRotator(oldEncoded, newEncoded)
	if err != nil {
		t.Fatalf("rotator: %v", err)
	}
	resealed, err := rotator.Reseal(sealedByOld)
	if err != nil {
		t.Fatalf("reseal: %v", err)
	}
	if resealed == sealedByOld {
		t.Fatal("the value was not re-sealed")
	}

	// The new key opens it; the old one no longer does.
	if opened, err := mustSealer(t, newEncoded).Open(resealed); err != nil || opened != "3174012345670001" {
		t.Fatalf("new key opened %q (%v)", opened, err)
	}
	if _, err := mustSealer(t, oldEncoded).Open(resealed); err == nil {
		t.Fatal("the old key still opens a re-sealed value")
	}

	// Empty stays empty: an absent identity number is not a secret, and
	// re-sealing nothing would make "not provided" indistinguishable from
	// "provided".
	if out, err := rotator.Reseal(""); err != nil || out != "" {
		t.Fatalf("empty re-sealed to %q (%v)", out, err)
	}
}

// Rotating a key onto itself would let somebody believe they had rotated when
// nothing happened — and then destroy the old key that is still the only one
// that works.
func TestRotatorRefusesUselessOrUnusableRotations(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key)

	if _, err := NewRotator(encoded, encoded); err == nil {
		t.Fatal("rotating a key onto itself was accepted")
	}
	if _, err := NewRotator("", encoded); err == nil {
		t.Fatal("rotating from no key was accepted")
	}
	if _, err := NewRotator(encoded, ""); err == nil {
		t.Fatal("rotating to no key was accepted")
	}
	if _, err := NewRotator("not-base64!!", encoded); err == nil {
		t.Fatal("an unusable old key was accepted")
	}
}

// A value the old key cannot open must stop the rotation rather than be
// skipped: stamping it with the new key would claim a key that cannot read it.
func TestRotatorRefusesValuesTheOldKeyCannotOpen(t *testing.T) {
	first, second, third := make([]byte, 32), make([]byte, 32), make([]byte, 32)
	for _, key := range [][]byte{first, second, third} {
		if _, err := rand.Read(key); err != nil {
			t.Fatalf("key: %v", err)
		}
	}
	// Sealed with a key that is neither side of the rotation.
	stranger, _ := mustSealer(t, base64.StdEncoding.EncodeToString(third)).Seal("3174012345670001")

	rotator, err := NewRotator(base64.StdEncoding.EncodeToString(first), base64.StdEncoding.EncodeToString(second))
	if err != nil {
		t.Fatalf("rotator: %v", err)
	}
	if _, err := rotator.Reseal(stranger); err == nil {
		t.Fatal("a value sealed by a third key was re-sealed anyway")
	}
}

func mustSealer(t *testing.T, encoded string) *Sealer {
	t.Helper()
	sealer, err := NewSealer(encoded)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	return sealer
}
