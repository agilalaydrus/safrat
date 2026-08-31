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

// Health reports contain sensitive medical information. Keep this test at the
// repository boundary: a future handler must not be able to accidentally
// bypass branch isolation.
func TestHealthReportRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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
	group := uuid.NewString()
	head := "health-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan)
	      VALUES ($1,$2,'Health scope','ID',$3,$4,'GROWTH')`, op, "health-scope-"+uuid.NewString(), op[:8]+"@example.test", "health-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, op) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO groups (id, operator_id, season_id, name) VALUES ($1,$2,$3,'Rombongan')`, group, op, season)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, group_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$3,$4,$5,$7,'Jamaah Bandung','HEALTH-BDG','ID','1990-01-01','MALE'),
	             ($2,$3,$4,$6,$7,'Jamaah Medan','HEALTH-MDN','ID','1991-01-01','FEMALE')`,
		bandungPilgrim, medanPilgrim, season, op, bandung, medan, group)
	exec(`INSERT INTO pilgrim_health_reports (id, operator_id, pilgrim_id, group_id, severity, symptoms)
	      VALUES ($1,$3,$4,$5,'BERAT','Bandung'),($2,$3,$6,$5,'BERAT','Medan')`, uuid.NewString(), uuid.NewString(), op, bandungPilgrim, group, medanPilgrim)

	repo := NewHealthReportRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, head)

	rows, err := repo.List(bandungCtx, op, nil)
	if err != nil || len(rows) != 1 || rows[0].PilgrimID != bandungPilgrim {
		t.Fatalf("list Bandung bocor atau gagal: %#v (%v)", rows, err)
	}
	if open, err := repo.HasOpenSevere(bandungCtx, op, bandungPilgrim); err != nil || !open {
		t.Fatalf("laporan Bandung tidak terlihat: open=%v err=%v", open, err)
	}
	if open, err := repo.HasOpenSevere(bandungCtx, op, medanPilgrim); err != nil || open {
		t.Fatalf("laporan Medan bocor ke Bandung: open=%v err=%v", open, err)
	}
	if _, err := repo.Create(bandungCtx, op, medanPilgrim, group, head, "RINGAN", "Tidak boleh", ""); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membuat laporan Medan: %v", err)
	}

	var medanReport string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM pilgrim_health_reports WHERE pilgrim_id = $1`, medanPilgrim).Scan(&medanReport); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Resolve(bandungCtx, op, medanReport); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menyelesaikan laporan Medan: %v", err)
	}
	var resolved bool
	if err := pool.QueryRow(ctx, `SELECT resolved FROM pilgrim_health_reports WHERE id = $1`, medanReport).Scan(&resolved); err != nil || resolved {
		t.Fatalf("laporan Medan berubah: resolved=%v err=%v", resolved, err)
	}
}
