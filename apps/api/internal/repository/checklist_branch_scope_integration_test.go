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

func TestChecklistRepositoryEnforcesBranchScopeIntegration(t *testing.T) {
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
	bandungHead := "checklist-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug)
	      VALUES ($1,$2,'Checklist Scope Test','ID',$3,$4)`, operatorID,
		"checklist-scope-"+uuid.NewString(), operatorID[:8]+"@example.test", "checklist-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, operatorID) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim Checklist','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$3,$4,$5,'Jamaah Bandung','CHK-BDG','ID','1990-01-01','MALE'),
	             ($2,$3,$4,$6,'Jamaah Medan','CHK-MDN','ID','1991-01-01','FEMALE')`,
		bandungPilgrimID, medanPilgrimID, seasonID, operatorID, bandungID, medanID)

	repo := NewChecklistRepository(db.New(pool))
	template, err := repo.CreateTemplate(ctx, operatorID, seasonID, "Paspor", "", "DOCUMENT", true, 1)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	bandungCtx := ContextWithStaffActor(ctx, bandungHead)
	if _, err := repo.UpsertItem(bandungCtx, template.ID, bandungPilgrimID, operatorID, true, bandungHead, "lengkap"); err != nil {
		t.Fatalf("kepala Bandung tidak dapat menandai jamaahnya: %v", err)
	}
	if _, err := repo.UpsertItem(bandungCtx, template.ID, medanPilgrimID, operatorID, true, bandungHead, "forbidden"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menandai jamaah Medan: %v", err)
	}
	items, err := repo.GetPilgrimChecklist(bandungCtx, operatorID, seasonID, medanPilgrimID)
	if err != nil || len(items) != 0 {
		t.Fatalf("kepala Bandung membaca checklist Medan: %#v (%v)", items, err)
	}
	stats, err := repo.GetStats(bandungCtx, operatorID, seasonID)
	if err != nil || len(stats) != 1 || stats[0].TotalPilgrims != 1 || stats[0].CompletedCount != 1 {
		t.Fatalf("statistik checklist Bandung bocor: %#v (%v)", stats, err)
	}
}
