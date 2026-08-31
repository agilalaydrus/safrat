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

func TestTransportRepositoryEnforcesManifestBranchScopeIntegration(t *testing.T) {
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

	op, season, movement := uuid.NewString(), uuid.NewString(), uuid.NewString()
	vehicle, secondVehicle := uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	head := "transport-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Transport scope','ID',$3,$4,'GROWTH')`, op, "transport-scope-"+uuid.NewString(), op[:8]+"@example.test", "transport-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,'Jamaah Bandung','BUS-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,'Jamaah Medan','BUS-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan)
	exec(`INSERT INTO movements (id,season_id,operator_id,name,origin,destination,scheduled_at) VALUES ($1,$2,$3,'Bus','Hotel','Bandara',NOW())`, movement, season, op)
	exec(`INSERT INTO vehicles (id,movement_id,operator_id,plate_number,capacity) VALUES ($1,$3,$4,'B-1',20),($2,$3,$4,'B-2',20)`, vehicle, secondVehicle, movement, op)
	exec(`INSERT INTO seat_assignments (vehicle_id,pilgrim_id,operator_id,seat_number,assigned_by) VALUES ($1,$2,$4,1,'fixture'),($1,$3,$4,2,'fixture')`, vehicle, bandungPilgrim, medanPilgrim, op)

	repo := NewTransportRepository(db.New(pool), pool)
	branchCtx := ContextWithStaffActor(ctx, head)
	_, manifest, err := repo.Manifest(branchCtx, op, vehicle)
	if err != nil || len(manifest) != 1 || manifest[0].ID != bandungPilgrim {
		t.Fatalf("manifest Bandung bocor: %#v (%v)", manifest, err)
	}
	if err := repo.UnassignSeat(branchCtx, op, vehicle, medanPilgrim); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung melepas kursi Medan: %v", err)
	}
	tx, err := repo.BeginTx(branchCtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AssignSeatTx(branchCtx, tx, op, secondVehicle, medanPilgrim, head, 3); !errors.Is(err, apperror.ErrNotFound) {
		_ = tx.Rollback(ctx)
		t.Fatalf("kepala Bandung memberi kursi Medan: %v", err)
	}
	_ = tx.Rollback(ctx)
	tx, err = repo.BeginTx(branchCtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AssignSeatTx(branchCtx, tx, op, secondVehicle, bandungPilgrim, head, 3); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("kepala Bandung gagal memberi kursi sendiri: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
