package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SOS exposes a pilgrim's emergency location. Both the list and state-changing
// actions must be branch-scoped at the repository, not merely at the RPC.
func TestSOSRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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

	op, season := uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	head := "sos-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan)
	      VALUES ($1,$2,'SOS scope','ID',$3,$4,'GROWTH')`, op, "sos-scope-"+uuid.NewString(), op[:8]+"@example.test", "sos-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, op) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO "user" (id, name, email, "emailVerified") VALUES ($1,'Kepala Bandung',$2,true)`, head, head+"@example.test")
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$3,$4,$5,'Jamaah Bandung','SOS-BDG','ID','1990-01-01','MALE'),
	             ($2,$3,$4,$6,'Jamaah Medan','SOS-MDN','ID','1991-01-01','FEMALE')`,
		bandungPilgrim, medanPilgrim, season, op, bandung, medan)
	bandungAlert, medanAlert := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO sos_alerts (id, operator_id, pilgrim_id, status) VALUES ($1,$3,$4,'ACTIVE'),($2,$3,$5,'ACTIVE')`, bandungAlert, medanAlert, op, bandungPilgrim, medanPilgrim)

	repo := NewSOSRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, head)
	alerts, err := repo.ListActive(bandungCtx, op)
	if err != nil || len(alerts) != 1 || alerts[0].PilgrimID != bandungPilgrim {
		t.Fatalf("SOS Bandung bocor atau gagal: %#v (%v)", alerts, err)
	}
	if _, err := repo.Acknowledge(bandungCtx, op, medanAlert, head); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengonfirmasi SOS Medan: %v", err)
	}
	if _, err := repo.Resolve(bandungCtx, op, medanAlert, head, "tidak boleh"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menyelesaikan SOS Medan: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM sos_alerts WHERE id = $1`, medanAlert).Scan(&status); err != nil || status != "ACTIVE" {
		t.Fatalf("SOS Medan berubah: status=%s err=%v", status, err)
	}
	if _, err := repo.Acknowledge(bandungCtx, op, bandungAlert, head); err != nil {
		t.Fatalf("kepala Bandung tidak dapat mengonfirmasi SOS sendiri: %v", err)
	}
}
