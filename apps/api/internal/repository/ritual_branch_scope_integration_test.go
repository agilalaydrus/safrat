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

func TestRitualRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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

	op, season, group, ritual := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	head := "ritual-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Ritual scope','ID',$3,$4,'GROWTH')`, op, "ritual-scope-"+uuid.NewString(), op[:8]+"@example.test", "ritual-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO groups (id,season_id,operator_id,name,capacity) VALUES ($1,$2,$3,'Grup Bersama',20)`, group, season, op)
	exec(`INSERT INTO ritual_templates (id,operator_id,season_type,name,description,order_num,is_required) VALUES ($1,$2,'UMRAH','Tawaf','Tujuh putaran',1,true)`, ritual, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,group_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,$6,'Jamaah Bandung','RTL-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$7,$6,'Jamaah Medan','RTL-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, group, medan)

	repo := NewRitualRepository(db.New(pool))
	branchCtx := ContextWithStaffActor(ctx, head)
	tx, err := pool.Begin(branchCtx)
	if err != nil {
		t.Fatal(err)
	}
	persistedScope, err := repo.BranchScopeIDTx(branchCtx, tx, op)
	_ = tx.Rollback(ctx)
	if err != nil || persistedScope != bandung {
		t.Fatalf("scope outbox ritual salah: %q (%v)", persistedScope, err)
	}
	progress, err := repo.GetGroupProgress(branchCtx, op, group)
	if err != nil || len(progress) != 1 || progress[0].TotalPilgrims != 1 || len(progress[0].IncompletePilgrimNames) != 1 || progress[0].IncompletePilgrimNames[0] != "Jamaah Bandung" {
		t.Fatalf("progres ritual Bandung bocor: %#v (%v)", progress, err)
	}
	if err = repo.CompletePilgrimRitual(branchCtx, op, medanPilgrim, ritual, "", "blocked"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menyelesaikan ritual Medan: %v", err)
	}
	if err = repo.CompletePilgrimRitual(branchCtx, op, bandungPilgrim, ritual, "", "allowed"); err != nil {
		t.Fatalf("kepala Bandung gagal menyelesaikan ritual sendiri: %v", err)
	}
	if _, err = repo.CreateTemplate(branchCtx, op, "UMRAH", "Lintas cabang", "", 2, true); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang membuat template operator-wide: %v", err)
	}
	headOffice, err := repo.GetGroupProgress(ctx, op, group)
	if err != nil || len(headOffice) != 1 || headOffice[0].TotalPilgrims != 2 || headOffice[0].CompletedCount != 1 || len(headOffice[0].IncompletePilgrimNames) != 1 || headOffice[0].IncompletePilgrimNames[0] != "Jamaah Medan" {
		t.Fatalf("progres kantor pusat salah: %#v (%v)", headOffice, err)
	}
}
