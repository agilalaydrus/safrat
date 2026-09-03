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

func TestRegistrationRepositoryEnforcesBranchScopeIntegration(t *testing.T) {
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

	operatorID, seasonID := uuid.NewString(), uuid.NewString()
	bandungID, medanID := uuid.NewString(), uuid.NewString()
	bandungAgentID, medanAgentID := uuid.NewString(), uuid.NewString()
	bandungHead := "registration-head-" + uuid.NewString()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan)
	      VALUES ($1,$2,'Registration Scope Test','ID',$3,$4,'GROWTH')`, operatorID,
		"reg-scope-"+uuid.NewString(), operatorID[:8]+"@example.test", "reg-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim Registrasi','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)
	exec(`INSERT INTO agents (id, operator_id, branch_id, name) VALUES ($1,$3,$4,'Agen Bandung'),($2,$3,$5,'Agen Medan')`, bandungAgentID, medanAgentID, operatorID, bandungID, medanID)

	repo := NewRegistrationRepository(db.New(pool))
	bandungRegistration, err := repo.Create(ctx, operatorID, seasonID, "", "Pendaftar Bandung", "REG-BDG", nil, "MALE", "", "", "ID", "", bandungAgentID, "", "")
	if err != nil {
		t.Fatalf("create Bandung: %v", err)
	}
	medanRegistration, err := repo.Create(ctx, operatorID, seasonID, "", "Pendaftar Medan", "REG-MDN", nil, "FEMALE", "", "", "ID", "", medanAgentID, "", "")
	if err != nil {
		t.Fatalf("create Medan: %v", err)
	}
	headOfficeRegistration, err := repo.Create(ctx, operatorID, seasonID, "", "Pendaftar Pusat", "REG-HQ", nil, "MALE", "", "", "ID", "", "", "", "")
	if err != nil {
		t.Fatalf("create head office: %v", err)
	}

	var bandungStored, medanStored string
	var headOfficeStored *string
	if err := pool.QueryRow(ctx, `SELECT branch_id::text FROM pilgrim_registrations WHERE id=$1`, bandungRegistration.ID).Scan(&bandungStored); err != nil {
		t.Fatalf("branch Bandung: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT branch_id::text FROM pilgrim_registrations WHERE id=$1`, medanRegistration.ID).Scan(&medanStored); err != nil {
		t.Fatalf("branch Medan: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT branch_id::text FROM pilgrim_registrations WHERE id=$1`, headOfficeRegistration.ID).Scan(&headOfficeStored); err != nil {
		t.Fatalf("branch pusat: %v", err)
	}
	if bandungStored != bandungID || medanStored != medanID || headOfficeStored != nil {
		t.Fatalf("cabang referral tidak diwarisi: Bandung=%s Medan=%s pusat=%v", bandungStored, medanStored, headOfficeStored)
	}

	bandungCtx := ContextWithStaffActor(ctx, bandungHead)
	rows, err := repo.List(bandungCtx, operatorID, seasonID)
	if err != nil || len(rows) != 1 || rows[0].ID != bandungRegistration.ID {
		t.Fatalf("inbox Bandung bocor lintas cabang: %#v (%v)", rows, err)
	}
	if _, err := repo.Get(bandungCtx, operatorID, medanRegistration.ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca registrasi Medan: %v", err)
	}
	if _, err := repo.UpdateStatus(bandungCtx, operatorID, medanRegistration.ID, "APPROVED", "forbidden"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menyetujui registrasi Medan: %v", err)
	}
	updated, err := repo.UpdateStatus(bandungCtx, operatorID, bandungRegistration.ID, "APPROVED", "ok")
	if err != nil || updated.Status != "APPROVED" {
		t.Fatalf("kepala Bandung tidak bisa menyetujui registrasinya: %#v (%v)", updated, err)
	}

	headOfficeCtx := ContextWithStaffActor(ctx, "registration-hq-"+uuid.NewString())
	rows, err = repo.List(headOfficeCtx, operatorID, seasonID)
	if err != nil || len(rows) != 3 {
		t.Fatalf("kantor pusat harus melihat semua registrasi: len=%d err=%v", len(rows), err)
	}
}
