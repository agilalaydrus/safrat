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

func TestStaffScheduleRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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
	bandungKloter, medanKloter := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	head := "schedule-branch-head-" + uuid.NewString()
	bandungStaff, medanStaff := "staff-bandung-"+uuid.NewString(), "staff-medan-"+uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Schedule scope','ID',$3,$4,'GROWTH')`, op, "schedule-scope-"+uuid.NewString(), op[:8]+"@example.test", "schedule-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO kloters (id,operator_id,season_id,code,capacity) VALUES ($1,$3,$4,'BDG-01',10),($2,$3,$4,'MDN-01',10)`, bandungKloter, medanKloter, op, season)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,kloter_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,$7,'Jamaah Bandung','SCH-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,$8,'Jamaah Medan','SCH-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan, bandungKloter, medanKloter)
	exec(`INSERT INTO kloter_staff (operator_id,kloter_id,staff_id,staff_name,staff_email,role) VALUES ($1,$2,$4,'Petugas Bandung','bandung@example.test','COORDINATOR'),($1,$3,$5,'Petugas Medan','medan@example.test','COORDINATOR')`, op, bandungKloter, medanKloter, bandungStaff, medanStaff)

	repo := NewStaffScheduleRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, head)

	rows, err := repo.ListForKloter(bandungCtx, op, bandungKloter)
	if err != nil || len(rows) != 1 || rows[0].StaffID != bandungStaff {
		t.Fatalf("kepala Bandung tidak melihat jadwalnya: %#v (%v)", rows, err)
	}
	rows, err = repo.ListForKloter(bandungCtx, op, medanKloter)
	if err != nil || len(rows) != 0 {
		t.Fatalf("jadwal dan email petugas Medan bocor ke Bandung: %#v (%v)", rows, err)
	}
	summaries, err := repo.ListAll(bandungCtx, op, season)
	if err != nil || len(summaries) != 1 || summaries[0].KloterID != bandungKloter {
		t.Fatalf("ringkasan jadwal Bandung bocor: %#v (%v)", summaries, err)
	}

	if _, err := repo.Assign(bandungCtx, op, bandungKloter, "new-staff", "Tidak boleh", "blocked@example.test", "COORDINATOR", ""); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang mengubah struktur petugas kloter: %v", err)
	}
	if err := repo.Remove(bandungCtx, bandungKloter, bandungStaff, op); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("kepala cabang menghapus petugas kloter: %v", err)
	}
	rows, err = repo.ListForKloter(ctx, op, bandungKloter)
	if err != nil || len(rows) != 1 {
		t.Fatalf("larangan mutasi tidak menjaga jadwal: %#v (%v)", rows, err)
	}

	summaries, err = repo.ListAll(ctx, op, season)
	if err != nil || len(summaries) != 2 {
		t.Fatalf("kantor pusat kehilangan jadwal operator: %#v (%v)", summaries, err)
	}
	rows, err = repo.ListForKloter(ctx, op, medanKloter)
	if err != nil || len(rows) != 1 || rows[0].StaffID != medanStaff {
		t.Fatalf("kantor pusat kehilangan jadwal Medan: %#v (%v)", rows, err)
	}
}
