package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func supplierCostFixture(t *testing.T) (*pgxpool.Pool, string, string) {
	t.Helper()
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

	operatorID, seasonID, productID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture %q: %v", query, err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Cost Uji','ID',$3,$4)`,
		operatorID, "cost-"+uuid.NewString(), operatorID[:8]+"@example.test", "cost-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, price_idr) VALUES ($1,$2,$3,'Paket Data',100000)`, productID, operatorID, seasonID)
	t.Cleanup(func() {
		cleanup, err := pool.Begin(context.Background())
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Rollback(context.Background()) }()
		if _, err := cleanup.Exec(context.Background(), `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM supplier_cost_observations WHERE operator_id = $1`, operatorID); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		_ = cleanup.Commit(context.Background())
	})
	return pool, operatorID, productID
}

func readCost(t *testing.T, pool *pgxpool.Pool, productID string) (*int64, string) {
	t.Helper()
	var cost *int64
	var source string
	if err := pool.QueryRow(context.Background(),
		`SELECT supplier_cost_idr, supplier_cost_source FROM products WHERE id = $1`, productID).Scan(&cost, &source); err != nil {
		t.Fatalf("read cost: %v", err)
	}
	return cost, source
}

func TestSupplierCostIsLearnedAndProtectedIntegration(t *testing.T) {
	pool, operatorID, productID := supplierCostFixture(t)
	ctx := context.Background()
	costs := NewSupplierCostRepository(pool)

	// Nothing known yet, so there is no floor to check a price against.
	if cost, source := readCost(t, pool, productID); cost != nil || source != "" {
		t.Fatalf("a fresh product already carries a cost: %v (%s)", cost, source)
	}

	// Entered by hand.
	if err := costs.SetManualCost(ctx, operatorID, productID, 80_000); err != nil {
		t.Fatalf("manual cost: %v", err)
	}
	cost, source := readCost(t, pool, productID)
	if cost == nil || *cost != 80_000 || source != "MANUAL" {
		t.Fatalf("cost = %v (%s), want 80000 MANUAL", cost, source)
	}

	// The first real fulfilment reports what the supplier actually charged,
	// which outranks the typed figure.
	if err := costs.RecordObservation(ctx, operatorID, productID, "", 85_000, "SUP-1"); err != nil {
		t.Fatalf("observe: %v", err)
	}
	cost, source = readCost(t, pool, productID)
	if cost == nil || *cost != 85_000 || source != "OBSERVED" {
		t.Fatalf("cost = %v (%s), want 85000 OBSERVED", cost, source)
	}

	// A stale manual entry must not overwrite what the supplier charged.
	if err := costs.SetManualCost(ctx, operatorID, productID, 10_000); err == nil {
		t.Fatal("a manual figure overwrote an observed supplier cost")
	}
	if cost, _ = readCost(t, pool, productID); cost == nil || *cost != 85_000 {
		t.Fatalf("observed cost changed to %v", cost)
	}

	// A later fulfilment keeps it current — a supplier raising their rate is
	// exactly what this has to notice.
	if err := costs.RecordObservation(ctx, operatorID, productID, "", 92_500, "SUP-2"); err != nil {
		t.Fatalf("second observation: %v", err)
	}
	if cost, _ = readCost(t, pool, productID); cost == nil || *cost != 92_500 {
		t.Fatalf("cost = %v after the supplier raised their rate, want 92500", cost)
	}

	// The history behind it survives, so a moving cost can be seen moving.
	var observations int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM supplier_cost_observations WHERE product_id = $1`, productID).Scan(&observations); err != nil {
		t.Fatalf("count: %v", err)
	}
	if observations != 2 {
		t.Fatalf("%d observations recorded, want 2", observations)
	}

	// And it is history: evidence of what a supplier charged cannot be edited.
	if _, err := pool.Exec(ctx, `UPDATE supplier_cost_observations SET cost_idr = 1 WHERE product_id = $1`, productID); err == nil {
		t.Fatal("a supplier cost observation was edited")
	}
}

// A retried fulfilment reports the same purchase, not a second one.
func TestSupplierCostObservationIsIdempotentPerOrderIntegration(t *testing.T) {
	pool, operatorID, productID := supplierCostFixture(t)
	ctx := context.Background()
	costs := NewSupplierCostRepository(pool)

	var seasonID, pilgrimID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM seasons WHERE operator_id = $1`, operatorID).Scan(&seasonID); err != nil {
		t.Fatalf("read season: %v", err)
	}
	pilgrimID = uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
		VALUES ($1,$2,$3,'Jamaah','P-C','ID','1990-01-01'::timestamptz,'MALE')`, pilgrimID, seasonID, operatorID); err != nil {
		t.Fatalf("insert pilgrim: %v", err)
	}
	var orderID string
	if err := pool.QueryRow(ctx, `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, unit_price_idr, total_price_idr, status)
		VALUES ($1,$2,$3,$4,100000,100000,'PAID') RETURNING id::text`, operatorID, seasonID, pilgrimID, productID).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	for i := 0; i < 4; i++ {
		if err := costs.RecordObservation(ctx, operatorID, productID, orderID, 70_000, "SUP-RETRY"); err != nil {
			t.Fatalf("observation %d: %v", i, err)
		}
	}
	var observations int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM supplier_cost_observations WHERE order_id = $1`, orderID).Scan(&observations); err != nil {
		t.Fatalf("count: %v", err)
	}
	if observations != 1 {
		t.Fatalf("%d observations for one fulfilment, want 1", observations)
	}
}
