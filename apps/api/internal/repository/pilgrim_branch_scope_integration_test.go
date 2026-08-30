package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is deliberately a repository test, not a handler test. A handler can
// forget a check; every caller still crosses this boundary before PostgreSQL
// returns personal data.
func TestPilgrimRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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
	bandungPilgrimID, medanPilgrimID := uuid.NewString(), uuid.NewString()
	bandungHead := "branch-head-" + uuid.NewString()
	headOfficeStaff := "head-office-" + uuid.NewString()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug)
	      VALUES ($1,$2,'Cabang Scope Test','ID',$3,$4)`,
		operatorID, "branch-scope-"+uuid.NewString(), operatorID[:8]+"@example.test", "scope-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim Scope','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$3,$4,$5,'Jamaah Bandung','SCOPE-BANDUNG','ID','1990-01-01','MALE'),
	             ($2,$3,$4,$6,'Jamaah Medan','SCOPE-MEDAN','ID','1991-01-01','FEMALE')`,
		bandungPilgrimID, medanPilgrimID, seasonID, operatorID, bandungID, medanID)

	repo := NewPilgrimRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, bandungHead)
	headOfficeCtx := ContextWithStaffActor(ctx, headOfficeStaff)

	// Positive direction: Bandung can see its own row.
	got, err := repo.Get(bandungCtx, operatorID, bandungPilgrimID)
	if err != nil || got.ID != bandungPilgrimID {
		t.Fatalf("kepala Bandung tidak bisa melihat jamaahnya sendiri: got=%v err=%v", got, err)
	}
	rows, err := repo.List(bandungCtx, operatorID, seasonID, 50, 0)
	if err != nil {
		t.Fatalf("list Bandung: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != bandungPilgrimID {
		t.Fatalf("list Bandung berisi jamaah lintas cabang: %#v", rows)
	}
	count, err := repo.CountBySeason(bandungCtx, operatorID, seasonID)
	if err != nil || count != 1 {
		t.Fatalf("hitungan Bandung = %d (%v), mau 1", count, err)
	}

	// Negative direction: asking directly for Medan must look exactly like a
	// missing row; revealing that the ID exists would itself leak information.
	if _, err := repo.Get(bandungCtx, operatorID, medanPilgrimID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca jamaah Medan: %v", err)
	}

	bandungDocumentID, medanDocumentID := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO pilgrim_documents (id, pilgrim_id, operator_id, doc_type, file_url, file_name, uploaded_by)
	      VALUES ($1,$3,$5,'PASSPORT','/bandung','bandung.pdf','fixture'),
	             ($2,$4,$5,'PASSPORT','/medan','medan.pdf','fixture')`,
		bandungDocumentID, medanDocumentID, bandungPilgrimID, medanPilgrimID, operatorID)
	documents, err := repo.ListDocuments(bandungCtx, operatorID, bandungPilgrimID)
	if err != nil || len(documents) != 1 || documents[0].ID != bandungDocumentID {
		t.Fatalf("dokumen Bandung tidak terbaca dengan benar: %#v (%v)", documents, err)
	}
	documents, err = repo.ListDocuments(bandungCtx, operatorID, medanPilgrimID)
	if err != nil || len(documents) != 0 {
		t.Fatalf("kepala Bandung membaca dokumen Medan: %#v (%v)", documents, err)
	}
	seasonDocuments, err := repo.ListSeasonDocuments(bandungCtx, operatorID, seasonID)
	if err != nil || len(seasonDocuments) != 1 || seasonDocuments[0].ID != bandungDocumentID {
		t.Fatalf("daftar dokumen musim bocor lintas cabang: %#v (%v)", seasonDocuments, err)
	}
	if _, err := repo.CreateDocument(bandungCtx, operatorID, medanPilgrimID, "VISA", "/forbidden", "forbidden.pdf", "test"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menambah dokumen Medan: %v", err)
	}
	if err := repo.DeleteDocument(bandungCtx, operatorID, medanDocumentID); err != nil {
		t.Fatalf("delete terlindungi seharusnya no-op, bukan error database: %v", err)
	}
	var medanDocumentStillExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pilgrim_documents WHERE id = $1)`, medanDocumentID).Scan(&medanDocumentStillExists); err != nil || !medanDocumentStillExists {
		t.Fatalf("kepala Bandung menghapus dokumen Medan: exists=%v err=%v", medanDocumentStillExists, err)
	}
	if _, err := repo.UpdateInsurance(bandungCtx, operatorID, medanPilgrimID, domain.PilgrimInsuranceInput{Provider: "Tidak boleh"}); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengubah asuransi Medan: %v", err)
	}

	// Head office is not in branch_members and remains operator-wide.
	rows, err = repo.List(headOfficeCtx, operatorID, seasonID, 50, 0)
	if err != nil || len(rows) != 2 {
		t.Fatalf("kantor pusat harus melihat dua cabang: len=%d err=%v", len(rows), err)
	}

	// New staff-created rows inherit the actor's branch inside the repository;
	// the handler and request cannot select a more privileged branch.
	created, err := repo.Create(bandungCtx, operatorID, domain.PilgrimInput{
		SeasonID: seasonID, FullName: "Jamaah Baru Bandung", Nationality: "ID",
		DateOfBirth: time.Date(1992, 1, 1, 0, 0, 0, 0, time.UTC), Gender: "MALE",
	})
	if err != nil {
		t.Fatalf("create Bandung: %v", err)
	}
	var storedBranch string
	if err := pool.QueryRow(ctx, `SELECT branch_id::text FROM pilgrims WHERE id = $1`, created.ID).Scan(&storedBranch); err != nil {
		t.Fatalf("read created branch: %v", err)
	}
	if storedBranch != bandungID {
		t.Fatalf("jamaah baru masuk cabang %s, mau Bandung %s", storedBranch, bandungID)
	}
}
