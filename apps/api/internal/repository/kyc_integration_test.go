package repository

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

// currentTestKey is what kycFixture installed, so a rotation test can rotate
// away from it. Package-level because the fixture returns a pool and ids, and
// threading a key through every caller for the one test that needs it would be
// noise.
var currentTestKey string

func kycFixture(t *testing.T) (*pgxpool.Pool, *KYCRepository, string, string) {
	t.Helper()
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}
	sealer, err := crypto.NewSealer(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	previous := kycSealer
	SetKYCSealer(sealer)
	currentTestKey = base64.StdEncoding.EncodeToString(key)
	t.Cleanup(func() { SetKYCSealer(previous) })

	operatorID, agentID := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'KYC Uji','ID',$3,$4)`,
		operatorID, "kyc-"+uuid.NewString(), operatorID[:8]+"@example.test", "kyc-"+operatorID[:8]); err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, operator_id, name) VALUES ($1,$2,'Agen KYC')`, agentID, operatorID); err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := pool.Begin(context.Background())
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Rollback(context.Background()) }()
		if _, err := cleanup.Exec(context.Background(), `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		_ = cleanup.Commit(context.Background())
	})
	return pool, NewKYCRepository(pool), operatorID, agentID
}

// The whole point: what a person submits must not be readable in the table.
func TestIdentityNumbersAreNotStoredInTheClearIntegration(t *testing.T) {
	pool, kyc, operatorID, agentID := kycFixture(t)
	ctx := context.Background()

	const nik = "3174012345670001"
	const npwp = "09.254.294.1-407.000"
	if _, err := kyc.Save(ctx, KYCRecord{
		OperatorID: operatorID, UserID: "user-kyc-1", SubjectType: "AGENT", SubjectID: agentID,
		FullName: "Agen KYC", NIK: nik, NPWP: npwp, Address: "Jl. Uji 1", Source: "SELF",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Read the raw column the way anybody with the database would.
	var storedNIK, storedNPWP string
	if err := pool.QueryRow(ctx,
		`SELECT nik_encrypted, npwp_encrypted FROM kyc_records WHERE subject_id = $1`, agentID).
		Scan(&storedNIK, &storedNPWP); err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if storedNIK == nik || storedNPWP == npwp {
		t.Fatal("an identity number is sitting in the table in the clear")
	}
	if !crypto.IsSealed(storedNIK) || !crypto.IsSealed(storedNPWP) {
		t.Fatalf("stored values are not sealed: %q / %q", storedNIK, storedNPWP)
	}

	// And the application still reads them back.
	record, err := kyc.ForSubject(ctx, "AGENT", agentID)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if record.NIK != nik || record.NPWP != npwp {
		t.Fatalf("read back %q / %q", record.NIK, record.NPWP)
	}
	// The relation to the account is what makes it retrievable on request.
	if record.UserID != "user-kyc-1" {
		t.Fatalf("user id = %q, want the account it belongs to", record.UserID)
	}
	byUser, err := kyc.ForUser(ctx, "user-kyc-1")
	if err != nil || len(byUser) != 1 {
		t.Fatalf("lookup by account returned %d records (%v)", len(byUser), err)
	}
}

// A verification applies to what was checked, not to whatever the person later
// replaced it with.
func TestResubmittingClearsAPriorVerificationIntegration(t *testing.T) {
	_, kyc, operatorID, agentID := kycFixture(t)
	ctx := context.Background()

	id, err := kyc.Save(ctx, KYCRecord{
		OperatorID: operatorID, SubjectType: "AGENT", SubjectID: agentID, NIK: "3174012345670001",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := kyc.SetStatus(ctx, id, "VERIFIED", "staff-1", ""); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Same subject, different number.
	if _, err := kyc.Save(ctx, KYCRecord{
		OperatorID: operatorID, SubjectType: "AGENT", SubjectID: agentID, NIK: "3174019999990002",
	}); err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	record, err := kyc.ForSubject(ctx, "AGENT", agentID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if record.Status != "PENDING_REVIEW" || record.VerifiedAt != nil {
		t.Fatalf("status=%s verifiedAt=%v — a prior verification survived a resubmission", record.Status, record.VerifiedAt)
	}
	if record.NIK != "3174019999990002" {
		t.Fatalf("the new number was not stored: %q", record.NIK)
	}
	// Still one record, not two: "which one is current" must stay answerable.
	if byUser, _ := kyc.List(ctx, "", 50); len(byUser) < 1 {
		t.Fatal("the record disappeared")
	}
}

// Existing plaintext has to move, and must not be left behind in both places.
func TestLegacyIdentitiesAreMovedAndClearedIntegration(t *testing.T) {
	pool, kyc, _, agentID := kycFixture(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `UPDATE agents SET nik = $2, npwp = $3 WHERE id = $1`,
		agentID, "3174011111110003", "01.234.567.8-999.000"); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	moved, err := kyc.MigrateLegacyIdentities(ctx, 200)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if moved < 1 {
		t.Fatal("nothing was moved")
	}

	// Gone from where it used to live.
	var legacyNIK, legacyNPWP string
	if err := pool.QueryRow(ctx, `SELECT nik, npwp FROM agents WHERE id = $1`, agentID).Scan(&legacyNIK, &legacyNPWP); err != nil {
		t.Fatalf("read legacy: %v", err)
	}
	if legacyNIK != "" || legacyNPWP != "" {
		t.Fatalf("the plaintext is still in the old column: %q / %q", legacyNIK, legacyNPWP)
	}

	// Present, encrypted, in its new home.
	record, err := kyc.ForSubject(ctx, "AGENT", agentID)
	if err != nil {
		t.Fatalf("read moved: %v", err)
	}
	if record.NIK != "3174011111110003" || record.NPWP != "01.234.567.8-999.000" {
		t.Fatalf("moved record reads %q / %q", record.NIK, record.NPWP)
	}
	var stored string
	if err := pool.QueryRow(ctx, `SELECT nik_encrypted FROM kyc_records WHERE subject_id = $1`, agentID).Scan(&stored); err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if !crypto.IsSealed(stored) {
		t.Fatal("the moved value was stored in the clear")
	}

	// Running again finds nothing left to do.
	if again, err := kyc.MigrateLegacyIdentities(ctx, 200); err != nil {
		t.Fatalf("second pass: %v", err)
	} else if again != 0 {
		t.Fatalf("a second pass moved %d records that were already moved", again)
	}
}

// After losing a key, the question is "what am I looking for". A fingerprint
// answers it without being the key, so a candidate can be checked before it is
// deployed rather than after.
func TestStoredRecordsNameTheKeyThatSealedThemIntegration(t *testing.T) {
	pool, kyc, operatorID, agentID := kycFixture(t)
	ctx := context.Background()

	if _, err := kyc.Save(ctx, KYCRecord{
		OperatorID: operatorID, SubjectType: "AGENT", SubjectID: agentID, NIK: "3174012345670011",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	var stamped string
	if err := pool.QueryRow(ctx,
		`SELECT key_fingerprint FROM kyc_records WHERE subject_id = $1`, agentID).Scan(&stamped); err != nil {
		t.Fatalf("read fingerprint: %v", err)
	}
	if stamped == "" {
		t.Fatal("a sealed record does not name the key that sealed it")
	}
	if stamped != KYCKeyFingerprint() {
		t.Fatalf("record stamped %q, key reports %q", stamped, KYCKeyFingerprint())
	}
	// It must not be the key, and must not be long enough to be useful as one.
	if len(stamped) > 16 {
		t.Fatalf("the fingerprint is suspiciously long: %q", stamped)
	}

	counts, err := kyc.KeyFingerprintsInUse(ctx)
	if err != nil {
		t.Fatalf("fingerprints in use: %v", err)
	}
	if counts[stamped] < 1 {
		t.Fatalf("the stored record is not counted against its own key: %v", counts)
	}

	// Loading a different key makes the mismatch visible rather than silent —
	// which is what turns "some records will not open" into "these records
	// need that key".
	other := newOtherSealer(t)
	if other.Fingerprint() == stamped {
		t.Fatal("two keys collided")
	}
}

func newOtherSealer(t *testing.T) *crypto.Sealer {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}
	sealer, err := crypto.NewSealer(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	return sealer
}

// Rotation over a real dataset is not one command — it is a command run until
// the old fingerprint's count reaches zero. So it has to be resumable, and
// running it again after it finishes has to do nothing.
func TestKeyRotationIsResumableAndIdempotentIntegration(t *testing.T) {
	pool, kyc, operatorID, agentID := kycFixture(t)
	ctx := context.Background()

	if _, err := kyc.Save(ctx, KYCRecord{
		OperatorID: operatorID, SubjectType: "AGENT", SubjectID: agentID,
		NIK: "3174012345670022", NPWP: "01.234.567.8-901.000",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	oldFingerprint := KYCKeyFingerprint()

	// A new key, and a rotator between them.
	newKeyBytes := make([]byte, 32)
	if _, err := rand.Read(newKeyBytes); err != nil {
		t.Fatalf("key: %v", err)
	}
	newKey := base64.StdEncoding.EncodeToString(newKeyBytes)
	rotator, err := crypto.NewRotator(currentTestKey, newKey)
	if err != nil {
		t.Fatalf("rotator: %v", err)
	}

	rotated, err := kyc.RotateKey(ctx, rotator, 100)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated < 1 {
		t.Fatal("nothing was rotated")
	}

	// The stamp moved with the value.
	var stamped string
	if err := pool.QueryRow(ctx, `SELECT key_fingerprint FROM kyc_records WHERE subject_id = $1`,
		agentID).Scan(&stamped); err != nil {
		t.Fatalf("read stamp: %v", err)
	}
	if stamped != rotator.ToFingerprint() {
		t.Fatalf("stamp = %q, want the new key's %q", stamped, rotator.ToFingerprint())
	}
	if stamped == oldFingerprint {
		t.Fatal("the stamp did not change")
	}

	// The new key reads it; nothing is lost in the move.
	newSealer, err := crypto.NewSealer(newKey)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	SetKYCSealer(newSealer)
	record, err := kyc.ForSubject(ctx, "AGENT", agentID)
	if err != nil {
		t.Fatalf("read after rotation: %v", err)
	}
	if record.NIK != "3174012345670022" || record.NPWP != "01.234.567.8-901.000" {
		t.Fatalf("after rotation the record reads %q / %q", record.NIK, record.NPWP)
	}

	// Running again finds nothing: it only selects rows still on the old key.
	if again, err := kyc.RotateKey(ctx, rotator, 100); err != nil {
		t.Fatalf("second pass: %v", err)
	} else if again != 0 {
		t.Fatalf("a second pass rotated %d already-rotated records", again)
	}

	// And the progress count is what tells somebody it is finished.
	counts, err := kyc.KeyFingerprintsInUse(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts[oldFingerprint] != 0 {
		t.Fatalf("%d records still hold the old key", counts[oldFingerprint])
	}
}
