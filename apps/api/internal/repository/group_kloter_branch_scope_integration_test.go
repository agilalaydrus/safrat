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

func TestGroupAndKloterRepositoriesEnforceBranchScopeIntegration(t *testing.T) {
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
	op, season := uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bdgGroup, mdnGroup := uuid.NewString(), uuid.NewString()
	bdgKloter, mdnKloter := uuid.NewString(), uuid.NewString()
	head := "group-branch-head-" + uuid.NewString()
	exec := func(q string, a ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Group scope','ID',$3,$4,'GROWTH')`, op, "group-scope-"+uuid.NewString(), op[:8]+"@example.test", "group-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO kloters (id,operator_id,season_id,code,capacity) VALUES ($1,$3,$4,'BDG',20),($2,$3,$4,'MDN',20)`, bdgKloter, mdnKloter, op, season)
	exec(`INSERT INTO groups (id,operator_id,season_id,name,kloter_id) VALUES ($1,$3,$4,'Bandung',$5),($2,$3,$4,'Medan',$6)`, bdgGroup, mdnGroup, op, season, bdgKloter, mdnKloter)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,group_id,kloter_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,$7,$9,'Jamaah Bandung','GROUP-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,$8,$10,'Jamaah Medan','GROUP-MDN','ID','1991-01-01','FEMALE')`, uuid.NewString(), uuid.NewString(), season, op, bandung, medan, bdgGroup, mdnGroup, bdgKloter, mdnKloter)
	branchCtx := ContextWithStaffActor(ctx, head)
	groups := NewGroupRepository(db.New(pool))
	kloters := NewKloterRepository(db.New(pool))
	gotGroups, err := groups.ListForOperator(branchCtx, op, season)
	if err != nil || len(gotGroups) != 1 || gotGroups[0].ID != bdgGroup {
		t.Fatalf("grup Bandung bocor: %#v %v", gotGroups, err)
	}
	gotKloters, err := kloters.ListForOperator(branchCtx, op, season)
	if err != nil || len(gotKloters) != 1 || gotKloters[0].ID != bdgKloter {
		t.Fatalf("kloter Bandung bocor: %#v %v", gotKloters, err)
	}
	roster, err := groups.GetRoster(branchCtx, op, mdnGroup)
	if err != nil || len(roster) != 0 {
		t.Fatalf("roster Medan bocor: %#v %v", roster, err)
	}
	if _, err := groups.Create(branchCtx, op, season, "Tidak boleh", 1); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang membuat grup: %v", err)
	}
	if _, err := kloters.Create(branchCtx, op, season, "NO", "", "", nil, 1); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang membuat kloter: %v", err)
	}
}
