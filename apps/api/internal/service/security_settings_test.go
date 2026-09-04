package service

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetIpAllowlist must refuse to enable a list that does not include the
// caller's own current IP — the one guard that stands between this feature
// and locking an operator out of their own account. Also proves
// RevokeSession is scoped to the calling operator: a session id that
// belongs to a different tenant must not be revocable from here.
func TestSecuritySettingsSelfLockoutAndTenantScopingIntegration(t *testing.T) {
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

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture %q: %v", query, err)
		}
	}

	operatorAID, orgAID := uuid.NewString(), "sec-a-"+uuid.NewString()
	operatorBID, orgBID := uuid.NewString(), "sec-b-"+uuid.NewString()
	userAID, userBID := uuid.NewString(), uuid.NewString()
	sessionAID, sessionBID := "sess-"+uuid.NewString(), "sess-"+uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Sec A','ID',$3,$4)`,
		operatorAID, orgAID, operatorAID[:8]+"@example.test", "sec-a-"+operatorAID[:8])
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Sec B','ID',$3,$4)`,
		operatorBID, orgBID, operatorBID[:8]+"@example.test", "sec-b-"+operatorBID[:8])
	exec(`INSERT INTO "user" (id, name, email, "emailVerified", "createdAt", "updatedAt") VALUES ($1,'User A','a-`+userAID+`@example.test',true,NOW(),NOW())`, userAID)
	exec(`INSERT INTO "user" (id, name, email, "emailVerified", "createdAt", "updatedAt") VALUES ($1,'User B','b-`+userBID+`@example.test',true,NOW(),NOW())`, userBID)
	exec(`INSERT INTO organization (id, name, slug, "createdAt") VALUES ($1,'Org A',$1,NOW())`, orgAID)
	exec(`INSERT INTO organization (id, name, slug, "createdAt") VALUES ($1,'Org B',$1,NOW())`, orgBID)
	exec(`INSERT INTO member (id, "organizationId", "userId", role, "createdAt") VALUES ($1,$2,$3,'owner',NOW())`, "mem-a-"+userAID, orgAID, userAID)
	exec(`INSERT INTO member (id, "organizationId", "userId", role, "createdAt") VALUES ($1,$2,$3,'owner',NOW())`, "mem-b-"+userBID, orgBID, userBID)
	exec(`INSERT INTO session (id, "expiresAt", token, "createdAt", "updatedAt", "userId") VALUES ($1,NOW()+INTERVAL '1 day','tok-a',NOW(),NOW(),$2)`, sessionAID, userAID)
	exec(`INSERT INTO session (id, "expiresAt", token, "createdAt", "updatedAt", "userId") VALUES ($1,NOW()+INTERVAL '1 day','tok-b',NOW(),NOW(),$2)`, sessionBID, userBID)

	t.Cleanup(func() {
		exec(`DELETE FROM session WHERE "userId" IN ($1,$2)`, userAID, userBID)
		exec(`DELETE FROM member WHERE "userId" IN ($1,$2)`, userAID, userBID)
		exec(`DELETE FROM organization WHERE id IN ($1,$2)`, orgAID, orgBID)
		exec(`DELETE FROM operator_security_settings WHERE operator_id IN ($1,$2)`, operatorAID, operatorBID)
		exec(`DELETE FROM operators WHERE id IN ($1,$2)`, operatorAID, operatorBID)
		exec(`DELETE FROM "user" WHERE id IN ($1,$2)`, userAID, userBID)
	})

	queries := db.New(pool)
	svc := NewSecuritySettingsService(repository.NewOperatorRepository(queries), repository.NewSecuritySettingsRepository(queries))

	// Refused: 203.0.113.5 is not inside 198.51.100.0/24.
	if _, err := svc.SetIpAllowlist(ctx, orgAID, userAID, "203.0.113.5", "owner", &hajjv1.SetIpAllowlistRequest{
		Enabled: true, Cidrs: []string{"198.51.100.0/24"},
	}); err == nil {
		t.Fatalf("SetIpAllowlist mengizinkan mengaktifkan daftar yang tidak mencakup IP pemanggil sendiri")
	}

	// Accepted: the caller's own IP is inside the range being saved.
	posture, err := svc.SetIpAllowlist(ctx, orgAID, userAID, "198.51.100.42", "owner", &hajjv1.SetIpAllowlistRequest{
		Enabled: true, Cidrs: []string{"198.51.100.0/24"},
	})
	if err != nil {
		t.Fatalf("SetIpAllowlist (valid): %v", err)
	}
	if !posture.IpAllowlistEnabled || len(posture.IpAllowlistCidrs) != 1 {
		t.Fatalf("posture tidak sesuai setelah diaktifkan: %+v", posture)
	}

	// Non-owner/admin must be refused outright, before the lockout check
	// even runs.
	if _, err := svc.SetIpAllowlist(ctx, orgAID, userAID, "198.51.100.42", "staff", &hajjv1.SetIpAllowlistRequest{
		Enabled: true, Cidrs: []string{"198.51.100.0/24"},
	}); err == nil {
		t.Fatalf("SetIpAllowlist mengizinkan peran staff mengubah kebijakan keamanan")
	}

	// Tenant scoping: operator A cannot revoke operator B's session by id.
	if err := svc.RevokeSession(ctx, orgAID, &hajjv1.RevokeSessionRequest{SessionId: sessionBID}); err == nil {
		t.Fatalf("RevokeSession mengizinkan operator A menghapus sesi milik operator B")
	}
	sessionsB, err := svc.ListActiveSessions(ctx, orgBID, "", &hajjv1.ListActiveSessionsRequest{})
	if err != nil {
		t.Fatalf("ListActiveSessions (B): %v", err)
	}
	if len(sessionsB.Sessions) != 1 || sessionsB.Sessions[0].Id != sessionBID {
		t.Fatalf("sesi operator B seharusnya masih ada dan utuh: %+v", sessionsB.Sessions)
	}

	// Same operator: revoking its own session succeeds.
	if err := svc.RevokeSession(ctx, orgAID, &hajjv1.RevokeSessionRequest{SessionId: sessionAID}); err != nil {
		t.Fatalf("RevokeSession (milik sendiri): %v", err)
	}
	sessionsA, err := svc.ListActiveSessions(ctx, orgAID, "", &hajjv1.ListActiveSessionsRequest{})
	if err != nil {
		t.Fatalf("ListActiveSessions (A): %v", err)
	}
	if len(sessionsA.Sessions) != 0 {
		t.Fatalf("sesi operator A seharusnya sudah tercabut: %+v", sessionsA.Sessions)
	}
}
