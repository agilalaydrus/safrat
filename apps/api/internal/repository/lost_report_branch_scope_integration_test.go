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

func TestLostReportRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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
	head := "lost-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan)
	 VALUES ($1,$2,'Lost scope','ID',$3,$4,'GROWTH')`, op, "lost-scope-"+uuid.NewString(), op[:8]+"@example.test", "lost-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, op) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	 VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, full_name, passport_number, nationality, date_of_birth, gender)
	 VALUES ($1,$3,$4,$5,'Jamaah Bandung','LOST-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,'Jamaah Medan','LOST-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan)
	bandungReport, medanReport := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO lost_reports (id, pilgrim_id, operator_id, latitude, longitude, last_known_location)
	 VALUES ($1,$3,$4,0,0,'Bandung'),($2,$5,$4,0,0,'Medan')`, bandungReport, medanReport, bandungPilgrim, op, medanPilgrim)

	repo := NewLostReportRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, head)
	reports, err := repo.ListActive(bandungCtx, op)
	if err != nil || len(reports) != 1 || reports[0].PilgrimID != bandungPilgrim {
		t.Fatalf("laporan Bandung bocor atau gagal: %#v (%v)", reports, err)
	}
	if err := repo.Resolve(bandungCtx, op, medanReport); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menemukan laporan Medan: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM lost_reports WHERE id = $1`, medanReport).Scan(&status); err != nil || status != "LOST" {
		t.Fatalf("laporan Medan berubah: status=%s err=%v", status, err)
	}
	if err := repo.Resolve(bandungCtx, op, bandungReport); err != nil {
		t.Fatalf("kepala Bandung tidak dapat menutup laporan sendiri: %v", err)
	}
}
