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

func TestWaitlistRepositoryKeepsUnassignedPIIAtHeadOfficeIntegration(t *testing.T) {
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

	op, season, branch := uuid.NewString(), uuid.NewString(), uuid.NewString()
	entry := uuid.NewString()
	head := "waitlist-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Waitlist scope','ID',$3,$4,'GROWTH')`, op, "waitlist-scope-"+uuid.NewString(), op[:8]+"@example.test", "waitlist-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',1)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$2,'Bandung','Bandung')`, branch, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, branch, op)
	exec(`INSERT INTO season_waitlists (id,operator_id,season_id,full_name,email,phone,position,status) VALUES ($1,$2,$3,'Calon Jamaah Pusat','pusat@example.test','081200000000',1,'WAITING')`, entry, op, season)

	repo := NewWaitlistRepository(db.New(pool))
	branchCtx := ContextWithStaffActor(ctx, head)
	if _, err := repo.List(branchCtx, op, season); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang membaca PII antrean pusat: %v", err)
	}
	if _, err := repo.Promote(branchCtx, op, entry); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang mempromosikan antrean pusat: %v", err)
	}
	if _, err := repo.AdminConfirm(branchCtx, op, entry); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang mengonfirmasi antrean pusat: %v", err)
	}
	if _, err := repo.PromoteNextWaiting(branchCtx, op, season); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang mempromosikan antrean pusat secara massal: %v", err)
	}
	if err := repo.Remove(branchCtx, op, entry); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang menghapus antrean pusat: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM season_waitlists WHERE id=$1`, entry).Scan(&status); err != nil || status != "WAITING" {
		t.Fatalf("baris pusat berubah setelah mutasi terlarang: status=%q err=%v", status, err)
	}
	rows, err := repo.List(ctx, op, season)
	if err != nil || len(rows) != 1 || rows[0].Email != "pusat@example.test" {
		t.Fatalf("kantor pusat kehilangan antreannya: %#v (%v)", rows, err)
	}
	if promoted, err := repo.PromoteNextWaiting(ctx, op, season); err != nil || promoted == nil || promoted.ID != entry {
		t.Fatalf("worker/kantor pusat tidak dapat mempromosikan antrean: %#v (%v)", promoted, err)
	}
}
