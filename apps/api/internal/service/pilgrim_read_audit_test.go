package service

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UU PDP art. 46 gives 72 hours to tell affected people what was exposed. That
// is answerable only if reads leave a trail — without one the honest answer is
// "everyone", and every jamaah has to be notified for a breach that touched a
// handful.
func TestReadingPersonalDataLeavesATrailIntegration(t *testing.T) {
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

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture %q: %v", query, err)
		}
	}
	operatorID, orgID := uuid.NewString(), "read-"+uuid.NewString()
	seasonID, pilgrimID := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Jejak Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "read-"+operatorID[:8])
	t.Cleanup(func() {
		bg := context.Background()
		tx, err := pool.Begin(bg)
		if err != nil {
			return
		}
		defer func() { _ = tx.Rollback(bg) }()
		if _, err := tx.Exec(bg, `SELECT set_config('app.allow_ledger_purge','on',true)`); err != nil {
			return
		}
		if _, err := tx.Exec(bg, `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		_ = tx.Commit(bg)
	})
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$2,$3,'Jamaah Jejak','P-TRAIL','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimID, seasonID, operatorID)

	queries := db.New(pool)
	pilgrims := NewPilgrimService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		repository.NewAccommodationRepository(queries), repository.NewTransportRepository(queries, pool),
		repository.NewAuditRepository(queries), pool)

	// The identity has to reach the trail: "somebody read this" is not an
	// answer a regulator or a jamaah can act on.
	staffCtx := middleware.ContextWithIdentity(ctx, "staf-uji", operatorID)

	if _, err := pilgrims.List(staffCtx, orgID, &hajjv1.ListPilgrimsRequest{SeasonId: seasonID}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if _, err := pilgrims.Get(staffCtx, orgID, &hajjv1.GetPilgrimRequest{PilgrimId: pilgrimID}); err != nil {
		t.Fatalf("get: %v", err)
	}

	var listed, read int
	var who string
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE action = 'pilgrims_listed'),
		        count(*) FILTER (WHERE action = 'pilgrim_read'),
		        COALESCE(max(user_id), '')
		 FROM audit_logs WHERE operator_id = $1`, operatorID).Scan(&listed, &read, &who); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if listed != 1 {
		t.Fatalf("%d catatan pembacaan massal, mau 1", listed)
	}
	if read != 1 {
		t.Fatalf("%d catatan pembacaan detail, mau 1", read)
	}
	if who != "staf-uji" {
		t.Fatalf("jejak tidak menyebut siapa yang membaca: %q", who)
	}

	// The detail read must name the subject — that is the precise answer a
	// notification needs, and the one nothing else supplies afterwards.
	var subject string
	if err := pool.QueryRow(ctx,
		`SELECT entity_id FROM audit_logs WHERE operator_id = $1 AND action = 'pilgrim_read'`,
		operatorID).Scan(&subject); err != nil {
		t.Fatalf("read subject: %v", err)
	}
	if subject != pilgrimID {
		t.Fatalf("subjek = %q, mau %q", subject, pilgrimID)
	}
}
