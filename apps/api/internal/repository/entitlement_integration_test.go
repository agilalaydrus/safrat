package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPlanEntitlementsAreDatabaseEnforcedIntegration(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug)
		VALUES ($1,$2,'Entitlement Test','ID',$3,$4)`, operatorID, "entitlement-"+uuid.NewString(), operatorID[:8]+"@example.test", "entitlement-"+operatorID[:8]); err != nil {
		t.Fatalf("operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, operatorID) })
	if _, err := pool.Exec(ctx, `INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
		VALUES ($1,$2,'Musim Entitlement','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',500)`, seasonID, operatorID); err != nil {
		t.Fatalf("season: %v", err)
	}
	insertPilgrim := func(n int) error {
		_, err := pool.Exec(ctx, `INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
			VALUES ($1,$2,$3,$4,$5,'ID','1990-01-01','MALE')`, uuid.NewString(), seasonID, operatorID, fmt.Sprintf("Jamaah %d", n), fmt.Sprintf("ENT-%03d", n))
		return err
	}
	for n := 1; n <= 200; n++ {
		if err := insertPilgrim(n); err != nil {
			t.Fatalf("jamaah ke-%d ditolak sebelum limit: %v", n, err)
		}
	}
	if err := insertPilgrim(201); !constraintIs(err, "operator_pilgrim_limit") {
		t.Fatalf("jamaah ke-201 harus diblokir limit STARTER: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$2,'Bandung','Bandung')`, uuid.NewString(), operatorID); !constraintIs(err, "operator_branch_feature") {
		t.Fatalf("cabang STARTER harus dikunci: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO plan_overrides (operator_id, max_pilgrims, max_branches, feature_flag_overrides)
		VALUES ($1,201,1,'{"branches":true}'::jsonb)`, operatorID); err != nil {
		t.Fatalf("override: %v", err)
	}
	if err := insertPilgrim(201); err != nil {
		t.Fatalf("override jamaah tidak berlaku: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$2,'Bandung','Bandung')`, uuid.NewString(), operatorID); err != nil {
		t.Fatalf("override cabang tidak berlaku: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$2,'Medan','Medan')`, uuid.NewString(), operatorID); !constraintIs(err, "operator_branch_limit") {
		t.Fatalf("cabang kedua harus diblokir override limit: %v", err)
	}
}

func constraintIs(err error, constraint string) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == constraint
}
