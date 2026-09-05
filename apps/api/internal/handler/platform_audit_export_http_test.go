package handler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func randomSigningKey(t *testing.T) (string, *crypto.Signer) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	signer, err := crypto.NewSigner(encoded)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return encoded, signer
}

// TestExportAuditTrailIsSignedStreamedAndTamperEvidentIntegration is C4
// (TUGAS-PANEL-SAAS.md) end to end over real HTTP: a platform admin exports
// the trail, the CSV bytes and the manifest arrive over the same stream, and
// the manifest's sha256/hmac actually prove something about those bytes —
// not just that a manifest-shaped message came back.
func TestExportAuditTrailIsSignedStreamedAndTamperEvidentIntegration(t *testing.T) {
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

	fixture := newHTTPFixture(t, pool)
	var userID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id, note) VALUES ($1, 'uji ekspor auditor')`, userID); err != nil {
		t.Fatalf("grant platform admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol 2fa: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})
	// A row this export must actually see, tied to the fixture's own operator
	// so the filter that matters (operator_id) has something real to match.
	_, _ = pool.Exec(ctx, `INSERT INTO audit_logs (operator_id, user_id, action, entity_type, entity_id)
		VALUES ($1, $2, 'export_test_marker', 'test', $3)`, fixture.operatorID, userID, fixture.orderID)

	keyB64, signer := randomSigningKey(t)
	_ = keyB64
	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries),
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool),
		repository.NewPersonalDataReadRepository(pool), signer, repository.NewSupportRepository(queries), repository.NewDataExportRepository(pool), repository.NewAnnouncementRepository(pool))
	path, serviceHandler := hajjv1connect.NewPlatformServiceHandler(
		handler.NewPlatformHandler(platform),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)

	request := connect.NewRequest(&hajjv1.ExportAuditTrailRequest{OperatorId: fixture.operatorID})
	request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	stream, err := client.ExportAuditTrail(ctx, request)
	if err != nil {
		t.Fatalf("ExportAuditTrail: %v", err)
	}
	var csvBytes bytes.Buffer
	var manifest *hajjv1.AuditExportManifest
	for stream.Receive() {
		msg := stream.Msg()
		if chunk := msg.GetCsvChunk(); chunk != nil {
			csvBytes.Write(chunk)
		}
		if m := msg.GetManifest(); m != nil {
			manifest = m
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream: %v", err)
	}
	_ = stream.Close()

	if manifest == nil {
		t.Fatal("tidak ada manifes yang diterima")
	}
	if !bytes.Contains(csvBytes.Bytes(), []byte("export_test_marker")) {
		t.Fatalf("CSV yang diterima tidak memuat baris yang seharusnya cocok dengan filter: %s", csvBytes.String())
	}

	// The core guarantee: recomputing SHA-256 over exactly the bytes received
	// must match the manifest — this is what an auditor would do first.
	sum := sha256.Sum256(csvBytes.Bytes())
	gotSHA := hex.EncodeToString(sum[:])
	if gotSHA != manifest.Sha256 {
		t.Fatalf("sha256 CSV yang diterima (%s) tidak cocok dengan manifes (%s)", gotSHA, manifest.Sha256)
	}

	// The signature guarantee: HMAC(sha256, key) must match, using the same
	// key the server was configured with (an auditor would be given this key
	// out of band, not fetch it from the app).
	mac := hmac.New(sha256.New, mustDecodeKey(t, keyB64))
	mac.Write([]byte(manifest.Sha256))
	wantHMAC := hex.EncodeToString(mac.Sum(nil))
	if wantHMAC != manifest.HmacSha256 {
		t.Fatalf("hmac di manifes tidak bisa diverifikasi dengan kunci yang sama")
	}
	if manifest.KeyFingerprint != signer.Fingerprint() {
		t.Fatalf("fingerprint di manifes (%s) tidak cocok dengan kunci server (%s)", manifest.KeyFingerprint, signer.Fingerprint())
	}

	// Tamper-evidence: a CSV that has been altered after the fact must no
	// longer match the manifest's sha256 — proving the manifest actually
	// certifies specific bytes, not just "an export happened."
	tampered := append(append([]byte(nil), csvBytes.Bytes()...), []byte("EXTRA-ROW-INJECTED")...)
	tamperedSum := sha256.Sum256(tampered)
	if hex.EncodeToString(tamperedSum[:]) == manifest.Sha256 {
		t.Fatal("CSV yang diubah seharusnya tidak lagi cocok dengan manifes")
	}
}

func mustDecodeKey(t *testing.T, b64 string) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	return raw
}

// TestExportAuditTrailRefusesWithoutSigningKeyIntegration proves the refusal
// is real: an unconfigured signer must not be silently skipped in favor of
// producing an unsigned export.
func TestExportAuditTrailRefusesWithoutSigningKeyIntegration(t *testing.T) {
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

	fixture := newHTTPFixture(t, pool)
	var userID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id, note) VALUES ($1, 'uji tanpa kunci')`, userID); err != nil {
		t.Fatalf("grant platform admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol 2fa: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries),
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool),
		repository.NewPersonalDataReadRepository(pool), nil, repository.NewSupportRepository(queries), repository.NewDataExportRepository(pool), repository.NewAnnouncementRepository(pool))
	path, serviceHandler := hajjv1connect.NewPlatformServiceHandler(
		handler.NewPlatformHandler(platform),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)

	request := connect.NewRequest(&hajjv1.ExportAuditTrailRequest{OperatorId: fixture.operatorID})
	request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	stream, err := client.ExportAuditTrail(ctx, request)
	if err != nil {
		t.Fatalf("ExportAuditTrail: %v", err)
	}
	for stream.Receive() {
	}
	if connect.CodeOf(stream.Err()) != connect.CodeFailedPrecondition {
		t.Fatalf("tanpa kunci penandatangan seharusnya ditolak, dapat %v (%s)", stream.Err(), connect.CodeOf(stream.Err()))
	}
}
