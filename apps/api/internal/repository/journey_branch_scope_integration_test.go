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

func TestJourneyRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
	url := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if url == "" { t.Skip("STOREFRONT_TEST_DATABASE_URL is not set") }
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil { t.Fatal(err) }
	t.Cleanup(pool.Close)

	op, season, kloter := uuid.NewString(), uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	head := "journey-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) { t.Helper(); if _, err := pool.Exec(ctx, query, args...); err != nil { t.Fatalf("fixture: %v", err) } }
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Journey scope','ID',$3,$4,'GROWTH')`, op, "journey-scope-"+uuid.NewString(), op[:8]+"@example.test", "journey-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO kloters (id,season_id,operator_id,code,departure_date,embarkation,capacity) VALUES ($1,$2,$3,'JRN-01',NOW(),'CGK',20)`, kloter, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,kloter_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,$6,'Jamaah Bandung','JRN-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$7,$6,'Jamaah Medan','JRN-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, kloter, medan)
	exec(`INSERT INTO pilgrim_journey_status (operator_id,pilgrim_id,status,notes) VALUES ($1,$2,'DEPARTED','Bandung'),($1,$3,'ARRIVED','Medan')`, op, bandungPilgrim, medanPilgrim)

	repo := NewJourneyRepository(db.New(pool))
	branchCtx := ContextWithStaffActor(ctx, head)
	statuses, err := repo.ListByKloter(branchCtx, op, kloter)
	if err != nil || len(statuses) != 1 || statuses[bandungPilgrim] != "DEPARTED" { t.Fatalf("status Bandung bocor: %#v (%v)", statuses, err) }
	counts, err := repo.CountByKloter(branchCtx, op, kloter)
	if err != nil || counts["DEPARTED"] != 1 || counts["ARRIVED"] != 0 { t.Fatalf("agregat Bandung bocor: %#v (%v)", counts, err) }
	if _, err = repo.GetStatus(branchCtx, op, medanPilgrim); !errors.Is(err, apperror.ErrNotFound) { t.Fatalf("kepala Bandung membaca perjalanan Medan: %v", err) }
	if _, err = repo.UpdateStatus(branchCtx, op, medanPilgrim, "ARRIVED", "RETURNED", "", "blocked"); !errors.Is(err, apperror.ErrNotFound) { t.Fatalf("kepala Bandung mengubah perjalanan Medan: %v", err) }
	if _, err = repo.UpdateStatus(branchCtx, op, bandungPilgrim, "DEPARTED", "ARRIVED", "", "allowed"); err != nil { t.Fatalf("kepala Bandung gagal mengubah jamaah sendiri: %v", err) }
	headOffice, err := repo.ListByKloter(ctx, op, kloter)
	if err != nil || len(headOffice) != 2 { t.Fatalf("kantor pusat harus melihat dua cabang: %#v (%v)", headOffice, err) }
}
