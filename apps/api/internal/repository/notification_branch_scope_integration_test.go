package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNotificationRepositoryFiltersGroupTokensByPersistedBranchScopeIntegration(t *testing.T) {
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
	op, otherOp, season, group := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Push scope','ID',$3,$4,'GROWTH'),($5,$6,'Other push','ID',$7,$8,'GROWTH')`, op, "push-scope-"+uuid.NewString(), op[:8]+"@example.test", "push-"+op[:8], otherOp, "push-other-"+uuid.NewString(), otherOp[:8]+"@example.test", "push-other-"+otherOp[:8])
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id IN ($1,$2)`, op, otherOp)
	})
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO groups (id,season_id,operator_id,name,capacity) VALUES ($1,$2,$3,'Grup Campuran',20)`, group, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,group_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,$6,'Jamaah Bandung','PSH-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$7,$6,'Jamaah Medan','PSH-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, group, medan)
	exec(`INSERT INTO pilgrim_push_tokens (operator_id,pilgrim_id,fcm_token) VALUES ($1,$2,'token-bandung'),($1,$3,'token-medan')`, op, bandungPilgrim, medanPilgrim)
	repo := NewNotificationRepository(db.New(pool))
	bandungTokens, err := repo.ListTokensForGroup(ctx, op, group, bandung)
	if err != nil || len(bandungTokens) != 1 || bandungTokens[0] != "token-bandung" {
		t.Fatalf("token Bandung bocor: %#v (%v)", bandungTokens, err)
	}
	allTokens, err := repo.ListTokensForGroup(ctx, op, group, "")
	if err != nil || len(allTokens) != 2 {
		t.Fatalf("event operator-wide kehilangan token: %#v (%v)", allTokens, err)
	}
	if err = repo.RegisterPilgrimToken(ctx, otherOp, bandungPilgrim, "cross-tenant-token"); err != nil {
		t.Fatalf("registrasi invalid seharusnya no-op aman: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM pilgrim_push_tokens WHERE fcm_token='cross-tenant-token'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("token lintas tenant tersimpan: %d (%v)", count, err)
	}
}
