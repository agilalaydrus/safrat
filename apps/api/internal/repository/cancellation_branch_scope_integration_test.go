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

func TestCancellationRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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
	head := "cancellation-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan) VALUES ($1,$2,'Cancellation scope','ID',$3,$4,'GROWTH')`, op, "cancellation-scope-"+uuid.NewString(), op[:8]+"@example.test", "cancel-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,'Jamaah Bandung','CAN-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,'Jamaah Medan','CAN-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan)
	medanCancellation := uuid.NewString()
	exec(`INSERT INTO pilgrim_cancellations (id,pilgrim_id,operator_id,season_id,reason,days_before,refund_pct,cancelled_by) VALUES ($1,$2,$3,$4,'Medan',1,0,'fixture')`, medanCancellation, medanPilgrim, op, season)

	repo := NewCancellationRepository(pool, db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, head)
	rows, err := repo.ListCancellations(bandungCtx, op, season)
	if err != nil || len(rows) != 0 {
		t.Fatalf("daftar pembatalan Bandung bocor: %#v (%v)", rows, err)
	}
	if _, err := repo.GetByPilgrimID(bandungCtx, op, medanPilgrim); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca pembatalan Medan: %v", err)
	}
	if _, err := repo.ConfirmCancellation(bandungCtx, op, medanPilgrim, season, "Tidak boleh", head, 1, 0, 0, 0, ""); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membatalkan jamaah Medan: %v", err)
	}
	if _, err := repo.ConfirmCancellation(bandungCtx, op, bandungPilgrim, season, "Bandung", head, 1, 0, 0, 0, ""); err != nil {
		t.Fatalf("kepala Bandung tidak dapat membatalkan jamaah sendiri: %v", err)
	}
}
