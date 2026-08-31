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

func TestAccommodationRepositoryEnforcesAllocationBranchScopeBothWaysIntegration(t *testing.T) {
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
	hotel, secondHotel := uuid.NewString(), uuid.NewString()
	room, secondRoom := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	bandungReplacement, medanReplacement := uuid.NewString(), uuid.NewString()
	head := "accommodation-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Accommodation scope','ID',$3,$4,'GROWTH')`, op, "accommodation-scope-"+uuid.NewString(), op[:8]+"@example.test", "accommodation-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES
		($1,$5,$6,$7,'Jamaah Bandung','ROOM-BDG','ID','1990-01-01','MALE'),
		($2,$5,$6,$8,'Jamaah Medan','ROOM-MDN','ID','1991-01-01','MALE'),
		($3,$5,$6,$7,'Pengganti Bandung','ROOM-BDG-R','ID','1992-01-01','MALE'),
		($4,$5,$6,$8,'Pengganti Medan','ROOM-MDN-R','ID','1993-01-01','MALE')`, bandungPilgrim, medanPilgrim, bandungReplacement, medanReplacement, season, op, bandung, medan)
	exec(`INSERT INTO hotels (id,operator_id,season_id,name,city) VALUES ($1,$3,$4,'Hotel Makkah','Makkah'),($2,$3,$4,'Hotel Madinah','Madinah')`, hotel, secondHotel, op, season)
	exec(`INSERT INTO rooms (id,hotel_id,operator_id,room_number,room_type,capacity,gender) VALUES ($1,$3,$4,'101','QUAD',4,'male'),($2,$5,$4,'201','QUAD',4,'male')`, room, secondRoom, hotel, op, secondHotel)
	exec(`INSERT INTO room_allocations (room_id,hotel_id,pilgrim_id,operator_id,assigned_by) VALUES ($1,$2,$3,$5,'fixture'),($1,$2,$4,$5,'fixture')`, room, hotel, bandungPilgrim, medanPilgrim, op)

	repo := NewAccommodationRepository(db.New(pool))
	branchCtx := ContextWithStaffActor(ctx, head)

	manifest, err := repo.ListAllocations(branchCtx, op, room)
	if err != nil || len(manifest) != 1 || manifest[0].PilgrimID != bandungPilgrim {
		t.Fatalf("manifest Bandung bocor: %#v (%v)", manifest, err)
	}
	assignments, err := repo.ListPilgrimRoomAssignments(branchCtx, op, season)
	if err != nil || len(assignments) != 1 || assignments[0].PilgrimID != bandungPilgrim {
		t.Fatalf("daftar penempatan Bandung bocor: %#v (%v)", assignments, err)
	}
	if _, err = repo.GetAllocationForHotel(branchCtx, op, medanPilgrim, hotel); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca alokasi Medan: %v", err)
	}
	if err = repo.Deallocate(branchCtx, op, medanPilgrim, room); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung melepas kamar Medan: %v", err)
	}
	if _, err = repo.Allocate(branchCtx, op, secondRoom, secondHotel, medanPilgrim, head); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menempatkan jamaah Medan: %v", err)
	}
	if _, err = repo.Allocate(branchCtx, op, secondRoom, secondHotel, bandungPilgrim, head); err != nil {
		t.Fatalf("kepala Bandung gagal menempatkan jamaah sendiri: %v", err)
	}
	count, err := repo.CountAllocated(branchCtx, op, room)
	if err != nil || count != 2 {
		t.Fatalf("kapasitas fisik tidak menghitung seluruh cabang: %d (%v)", count, err)
	}

	tx, err := pool.Begin(branchCtx)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.TransferAllocationTx(branchCtx, tx, bandungPilgrim, medanReplacement, op); !errors.Is(err, apperror.ErrNotFound) {
		_ = tx.Rollback(ctx)
		t.Fatalf("kepala Bandung mentransfer alokasi ke Medan: %v", err)
	}
	_ = tx.Rollback(ctx)

	tx, err = pool.Begin(branchCtx)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.TransferAllocationTx(branchCtx, tx, bandungPilgrim, bandungReplacement, op); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("kepala Bandung gagal mentransfer alokasi sendiri: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	headOffice, err := repo.ListPilgrimRoomAssignments(ctx, op, season)
	if err != nil || len(headOffice) != 3 {
		t.Fatalf("kantor pusat harus melihat seluruh alokasi: %d (%v)", len(headOffice), err)
	}
}
