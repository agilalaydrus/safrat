package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAuditRepositoryFreezesAndEnforcesBranchScopeIntegration(t *testing.T) {
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

	op := uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	actor := "audit-moving-staff-" + uuid.NewString()
	bandungReader := "audit-bandung-reader-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Audit scope','ID',$3,$4,'GROWTH')`, op, "audit-scope-"+uuid.NewString(), op[:8]+"@example.test", "audit-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, actor, bandung, op)

	queries := db.New(pool)
	audits := NewAuditRepository(queries)
	operators := NewOperatorRepository(queries)
	if err := audits.Write(ctx, op, actor, "pilgrim_read", "pilgrim", uuid.NewString(), "Jamaah Bandung dibaca"); err != nil {
		t.Fatalf("tulis log Bandung: %v", err)
	}

	// Moving the actor must affect only future facts. Recomputing branch at
	// read time would silently rewrite the meaning of the first log.
	exec(`UPDATE branch_members SET branch_id=$2 WHERE better_auth_user_id=$1`, actor, medan)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, bandungReader, bandung, op)
	if err := audits.Write(ctx, op, actor, "pilgrim_read", "pilgrim", uuid.NewString(), "Jamaah Medan dibaca"); err != nil {
		t.Fatalf("tulis log Medan: %v", err)
	}

	bandungLogs, err := operators.ListAuditLogs(ContextWithStaffActor(ctx, bandungReader), op, 20)
	if err != nil || len(bandungLogs) != 1 || bandungLogs[0].Description != "Jamaah Bandung dibaca" {
		t.Fatalf("riwayat Bandung berubah atau bocor: %#v (%v)", bandungLogs, err)
	}
	medanLogs, err := operators.ListAuditLogs(ContextWithStaffActor(ctx, actor), op, 20)
	if err != nil || len(medanLogs) != 1 || medanLogs[0].Description != "Jamaah Medan dibaca" {
		t.Fatalf("riwayat Medan berubah atau bocor: %#v (%v)", medanLogs, err)
	}
	headOfficeLogs, err := operators.ListAuditLogs(ctx, op, 20)
	if err != nil || len(headOfficeLogs) != 2 {
		t.Fatalf("kantor pusat kehilangan audit operator: %#v (%v)", headOfficeLogs, err)
	}

	var bandungCount, medanCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE branch_id=$2), COUNT(*) FILTER (WHERE branch_id=$3) FROM audit_logs WHERE operator_id=$1`, op, bandung, medan).Scan(&bandungCount, &medanCount); err != nil {
		t.Fatalf("baca cabang audit: %v", err)
	}
	if bandungCount != 1 || medanCount != 1 {
		t.Fatalf("cabang audit tidak dibekukan saat append: Bandung=%d Medan=%d", bandungCount, medanCount)
	}
}
