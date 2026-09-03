package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The global trail, filtered the way an incident is actually investigated.
//
// The filters are the point. During an incident nobody scrolls: they ask "what
// irreversible things happened", "who impersonated a customer", and "who read
// somebody's identity" — and a filter that quietly returns everything answers
// none of those while looking like it did.
func TestAuditTrailFiltersByCategoryTenantAndActorIntegration(t *testing.T) {
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
			t.Fatalf("fixture: %v", err)
		}
	}
	suffix := uuid.NewString()[:8]
	operatorA, operatorB := uuid.NewString(), uuid.NewString()
	for id, tag := range map[string]string{operatorA: "auda", operatorB: "audb"} {
		exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
			VALUES ($1,$2,$3,'ID',$4,$5)`, id, tag+"-"+suffix, "Uji "+tag,
			tag+"-"+suffix+"@example.test", tag+"-"+suffix)
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, id) })
	}
	actor := "audit-user-" + suffix
	exec(`INSERT INTO "user" (id,name,email,"emailVerified") VALUES ($1,'Aktor Uji',$2,true)`,
		actor, actor+"@example.test")
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, actor) })

	write := func(operatorID, action string) {
		exec(`INSERT INTO audit_logs (operator_id,user_id,action,entity_type,entity_id,metadata)
			VALUES ($1,$2,$3,'uji',$4,$5)`, operatorID, actor, action, uuid.NewString(),
			map[string]any{"message": "uji " + action})
	}
	write(operatorA, "tenant_suspended")
	write(operatorA, "impersonation_started")
	write(operatorA, "kyc_record_read")
	write(operatorA, "crm_lead_created")
	write(operatorB, "tenant_suspended")

	repo := NewPlatformRepository(pool)
	mine := func(entries []AuditEntry) []AuditEntry {
		kept := make([]AuditEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.ActorID == actor {
				kept = append(kept, entry)
			}
		}
		return kept
	}

	all, err := repo.AuditTrail(ctx, AuditFilter{Actor: actor, Limit: 200})
	if err != nil {
		t.Fatalf("audit semua: %v", err)
	}
	if len(mine(all)) != 5 {
		t.Fatalf("%d entri untuk aktor ini, mau 5", len(mine(all)))
	}

	privileged, err := repo.AuditTrail(ctx, AuditFilter{Actor: actor, Category: AuditCategoryPrivileged, Limit: 200})
	if err != nil {
		t.Fatalf("audit istimewa: %v", err)
	}
	for _, entry := range mine(privileged) {
		if entry.Action != "tenant_suspended" {
			t.Fatalf("saringan tindakan istimewa meloloskan %q", entry.Action)
		}
	}
	if len(mine(privileged)) != 2 {
		t.Fatalf("%d tindakan istimewa, mau 2", len(mine(privileged)))
	}

	impersonation, err := repo.AuditTrail(ctx, AuditFilter{Actor: actor, Category: AuditCategoryImpersonate, Limit: 200})
	if err != nil {
		t.Fatalf("audit impersonasi: %v", err)
	}
	if len(mine(impersonation)) != 1 || mine(impersonation)[0].Action != "impersonation_started" {
		t.Fatalf("saringan impersonasi salah: %+v", mine(impersonation))
	}

	personal, err := repo.AuditTrail(ctx, AuditFilter{Actor: actor, Category: AuditCategoryPersonalData, Limit: 200})
	if err != nil {
		t.Fatalf("audit data pribadi: %v", err)
	}
	if len(mine(personal)) != 1 || mine(personal)[0].Action != "kyc_record_read" {
		t.Fatalf("saringan data pribadi salah: %+v", mine(personal))
	}

	// One tenant at a time, which is the other half of an incident question.
	perTenant, err := repo.AuditTrail(ctx, AuditFilter{OperatorID: operatorB, Limit: 200})
	if err != nil {
		t.Fatalf("audit per tenant: %v", err)
	}
	for _, entry := range perTenant {
		if entry.OperatorID != operatorB {
			t.Fatalf("saringan tenant meloloskan travel lain: %+v", entry)
		}
	}
	if len(mine(perTenant)) != 1 {
		t.Fatalf("%d entri untuk travel B, mau 1", len(mine(perTenant)))
	}

	// The actor is searchable by email, because an incident starts from a
	// person and nobody remembers a Better Auth user id.
	byEmail, err := repo.AuditTrail(ctx, AuditFilter{Actor: actor + "@example", Limit: 200})
	if err != nil {
		t.Fatalf("audit per email: %v", err)
	}
	if len(mine(byEmail)) != 5 {
		t.Fatalf("pencarian lewat email menemukan %d, mau 5", len(mine(byEmail)))
	}

	// A time window that excludes everything must return nothing rather than
	// silently ignoring the filter.
	future := time.Now().Add(time.Hour)
	empty, err := repo.AuditTrail(ctx, AuditFilter{Actor: actor, Since: &future, Limit: 200})
	if err != nil {
		t.Fatalf("audit rentang waktu: %v", err)
	}
	if len(mine(empty)) != 0 {
		t.Fatalf("saringan waktu diabaikan: %d entri", len(mine(empty)))
	}
}
