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

func TestBroadcastRepositoryKeepsOperatorWideMutationsAtHeadOfficeIntegration(t *testing.T) {
	url := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	op, season, branch := uuid.NewString(), uuid.NewString(), uuid.NewString()
	head := "broadcast-head-" + uuid.NewString()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Broadcast scope','ID',$3,$4,'GROWTH')`, op, "broadcast-scope-"+uuid.NewString(), op[:8]+"@example.test", "broadcast-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$2,'Bandung','Bandung')`, branch, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, branch, op)
	repo := NewBroadcastRepository(db.New(pool))
	branchCtx := ContextWithStaffActor(ctx, head)
	if _, err = repo.Create(branchCtx, op, season, "Tidak boleh", "Lintas cabang"); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang membuat broadcast pusat: %v", err)
	}
	created, err := repo.Create(ctx, op, season, "Pengumuman pusat", "Untuk semua jamaah")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := repo.List(branchCtx, op, season)
	if err != nil || len(rows) != 1 || rows[0].ID != created.ID {
		t.Fatalf("kepala cabang gagal membaca broadcast pusat: %#v (%v)", rows, err)
	}
	if err = repo.Delete(branchCtx, op, created.ID); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang menghapus broadcast pusat: %v", err)
	}
	if err = repo.Delete(ctx, op, created.ID); err != nil {
		t.Fatalf("kantor pusat gagal menghapus broadcast: %v", err)
	}
}
