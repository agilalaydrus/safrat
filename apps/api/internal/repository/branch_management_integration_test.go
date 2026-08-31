package repository

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func TestBranchManagementGuardsIntegration(t *testing.T) {
	url := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	op := uuid.NewString()
	exec := func(q string, a ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, a...); e != nil {
			t.Fatalf("fixture: %v", e)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Branch management','ID',$3,$4,'GROWTH')`, op, "branch-mgmt-"+uuid.NewString(), op[:8]+"@example.test", "branch-mgmt-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	repo := NewBranchRepository(db.New(pool))
	bandung, e := repo.Create(ctx, op, domain.Branch{Name: "Bandung", City: "Bandung", IsActive: true})
	if e != nil {
		t.Fatal(e)
	}
	medan, e := repo.Create(ctx, op, domain.Branch{Name: "Medan", City: "Medan", IsActive: true})
	if e != nil {
		t.Fatal(e)
	}
	head := "branch-head-" + uuid.NewString()
	if e = repo.AssignHead(ctx, op, bandung.ID, head); e != nil {
		t.Fatal(e)
	}
	if e = repo.AssignHead(ctx, op, medan.ID, head); !errors.Is(e, apperror.ErrAlreadyExists) {
		t.Fatalf("satu kepala memimpin dua cabang: %v", e)
	}
	if _, e = repo.Create(ContextWithStaffActor(ctx, head), op, domain.Branch{Name: "Tidak Boleh", IsActive: true}); !errors.Is(e, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang membuat cabang: %v", e)
	}
	rows, e := repo.List(ContextWithStaffActor(ctx, head), op, true)
	if e != nil || len(rows) != 1 || rows[0].ID != bandung.ID {
		t.Fatalf("list kepala cabang bocor: %#v %v", rows, e)
	}
}
