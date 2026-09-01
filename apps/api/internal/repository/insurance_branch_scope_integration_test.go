package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInsuranceRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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
	head := "insurance-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Insurance scope','ID',$3,$4,'GROWTH')`, op, "insurance-scope-"+uuid.NewString(), op[:8]+"@example.test", "insurance-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,'Jamaah Bandung','INS-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,'Jamaah Medan','INS-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan)

	repo := NewInsuranceRepository(db.New(pool))
	bandungClaim, err := repo.CreateClaim(ctx, bandungPilgrim, op, "MEDICAL", time.Now(), "Bandung", 100000, "fixture")
	if err != nil {
		t.Fatalf("buat klaim Bandung: %v", err)
	}
	medanClaim, err := repo.CreateClaim(ctx, medanPilgrim, op, "MEDICAL", time.Now(), "Medan", 200000, "fixture")
	if err != nil {
		t.Fatalf("buat klaim Medan: %v", err)
	}

	bandungCtx := ContextWithStaffActor(ctx, head)
	rows, err := repo.ListClaims(bandungCtx, op)
	if err != nil || len(rows) != 1 || rows[0].ID != bandungClaim.ID {
		t.Fatalf("daftar klaim Bandung bocor: %#v (%v)", rows, err)
	}
	if _, err := repo.CreateClaim(bandungCtx, medanPilgrim, op, "MEDICAL", time.Now(), "Tidak boleh", 1, head); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membuat klaim Medan: %v", err)
	}
	if _, err := repo.UpdateClaimStatus(bandungCtx, op, medanClaim.ID, "PROCESSING", 0); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengubah klaim Medan: %v", err)
	}
	if _, err := repo.GetExportData(bandungCtx, medanClaim.ID, op); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengekspor data medis Medan: %v", err)
	}
	if _, err := repo.UpdateClaimStatus(bandungCtx, op, bandungClaim.ID, "PROCESSING", 0); err != nil {
		t.Fatalf("kepala Bandung tidak dapat memproses klaim sendiri: %v", err)
	}
	rows, err = repo.ListClaims(ctx, op)
	if err != nil || len(rows) != 2 {
		t.Fatalf("kantor pusat kehilangan klaim: %#v (%v)", rows, err)
	}
}
