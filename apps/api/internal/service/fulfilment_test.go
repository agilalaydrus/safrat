package service

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fulfilmentFixture struct {
	pool        *pgxpool.Pool
	service     *FulfilmentService
	fulfilments *repository.FulfilmentRepository
	operatorID  string
	orderID     string
	productID   string
	supplierID  string
	token       string
}

// A paid digital order, a supplier that knows how to answer, and a route
// between them.
func newFulfilmentFixture(t *testing.T) *fulfilmentFixture {
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

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture %q: %v", query, err)
		}
	}
	operatorID, seasonID := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Fulfil Uji','ID',$3,$4)`,
		operatorID, "fulfil-"+uuid.NewString(), operatorID[:8]+"@example.test", "fulfil-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID)

	pilgrimID, productID := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$2,$3,'Jamaah Fulfil','P-FUL','ID','1990-01-01'::timestamptz,'MALE')`, pilgrimID, seasonID, operatorID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, category, price_idr) VALUES ($1,$2,$3,'Pulsa 10K','PPOB_CREDIT',11000)`,
		productID, operatorID, seasonID)

	supplierID, token := uuid.NewString(), "cb-"+uuid.NewString()
	exec(`INSERT INTO suppliers (id, name, code, status, callback_token) VALUES ($1,'Supplier Fulfil',$2,'ACTIVE',$3)`,
		supplierID, "fulfil-"+uuid.NewString()[:8], token)
	exec(`INSERT INTO supplier_response_rules (supplier_id, priority, pattern, outcome, reference_group, cost_group)
	      VALUES ($1, 10, $2, 'SUCCESS', 'ref', 'cost')`,
		supplierID, `(?i)"status"\s*:\s*"OK".*?"sn"\s*:\s*"(?P<ref>[^"]+)".*?"harga"\s*:\s*(?P<cost>[0-9.]+)`)
	exec(`INSERT INTO supplier_response_rules (supplier_id, priority, pattern, outcome)
	      VALUES ($1, 20, $2, 'FAILED')`, supplierID, `(?i)(gagal|failed)`)
	exec(`INSERT INTO product_routes (product_id, supplier_id, supplier_sku) VALUES ($1,$2,'PULSA10')`, productID, supplierID)
	t.Cleanup(func() {
		cleanup, err := pool.Begin(context.Background())
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Rollback(context.Background()) }()
		if _, err := cleanup.Exec(context.Background(), `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
			return
		}
		for _, statement := range []string{
			`DELETE FROM supplier_request_logs WHERE supplier_id = $1`,
			`DELETE FROM supplier_response_rules WHERE supplier_id = $1`,
		} {
			if _, err := cleanup.Exec(context.Background(), statement, supplierID); err != nil {
				return
			}
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM suppliers WHERE id = $1`, supplierID); err != nil {
			return
		}
		_ = cleanup.Commit(context.Background())
	})

	var orderID string
	if err := pool.QueryRow(ctx, `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity,
		unit_price_idr, total_price_idr, status, paid_at, paid_amount_idr)
		VALUES ($1,$2,$3,$4,1,11000,11000,'PAID',NOW(),11000) RETURNING id::text`,
		operatorID, seasonID, pilgrimID, productID).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	queries := db.New(pool)
	fulfilments := repository.NewFulfilmentRepository(pool)
	service := NewFulfilmentService(fulfilments, repository.NewSupplierRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewOrderRepository(queries))

	return &fulfilmentFixture{pool: pool, service: service, fulfilments: fulfilments,
		operatorID: operatorID, orderID: orderID, productID: productID, supplierID: supplierID, token: token}
}

func (f *fulfilmentFixture) status(t *testing.T) (string, string) {
	t.Helper()
	var status, reference string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status, supplier_reference FROM order_fulfilments WHERE order_id = $1`, f.orderID).Scan(&status, &reference); err != nil {
		t.Fatalf("read fulfilment: %v", err)
	}
	return status, reference
}

// A delivered transaction: the supplier answers, the rules read it, the state
// moves, and what they charged is learned from the same message.
func TestCallbackDeliversAndLearnsTheCostIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()
	f.service.Open(ctx, f.orderID, f.operatorID)

	if status, _ := f.status(t); status != "PENDING" {
		t.Fatalf("status = %s after opening, want PENDING", status)
	}

	result, err := f.service.ApplyCallback(ctx, f.token, f.orderID,
		`{"status":"OK","sn":"SN-4471","harga":9.900}`)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if result.Status != "DELIVERED" {
		t.Fatalf("status = %s, want DELIVERED", result.Status)
	}
	status, reference := f.status(t)
	if status != "DELIVERED" || reference != "SN-4471" {
		t.Fatalf("stored status=%s reference=%s, want DELIVERED and SN-4471", status, reference)
	}

	// The supplier told us what they charged, so the product now has a price
	// floor it did not have before.
	var cost *int64
	var source string
	if err := f.pool.QueryRow(ctx, `SELECT supplier_cost_idr, supplier_cost_source FROM products WHERE id = $1`,
		f.productID).Scan(&cost, &source); err != nil {
		t.Fatalf("read cost: %v", err)
	}
	if cost == nil || *cost != 9_900 || source != "OBSERVED" {
		t.Fatalf("cost = %v (%s), want 9900 OBSERVED", cost, source)
	}

	// And the exchange is on the record, raw body included.
	var logged int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM supplier_request_logs WHERE order_id = $1`, f.orderID).Scan(&logged); err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if logged != 1 {
		t.Fatalf("%d log entries, want 1", logged)
	}
}

// A supplier repeating itself must not deliver twice, and must not overwrite a
// settled state.
func TestCallbackIsIdempotentUnderConcurrencyIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()
	f.service.Open(ctx, f.orderID, f.operatorID)

	const attempts = 6
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.service.ApplyCallback(ctx, f.token, f.orderID,
				`{"status":"OK","sn":"SN-1","harga":9.900}`); err != nil {
				t.Errorf("callback: %v", err)
			}
		}()
	}
	wg.Wait()

	if status, _ := f.status(t); status != "DELIVERED" {
		t.Fatalf("status = %s, want DELIVERED", status)
	}
	// One purchase observed, however many times the supplier said so.
	var observations int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM supplier_cost_observations WHERE order_id = $1`, f.orderID).Scan(&observations); err != nil {
		t.Fatalf("count observations: %v", err)
	}
	if observations != 1 {
		t.Fatalf("%d cost observations from %d callbacks, want 1", observations, attempts)
	}
}

// The case that must never become a refund: a supplier saying something no rule
// recognises. It waits for a human instead.
func TestUnreadableCallbackNeedsReviewRatherThanFailingIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()
	f.service.Open(ctx, f.orderID, f.operatorID)

	result, err := f.service.ApplyCallback(ctx, f.token, f.orderID, "OK 4711")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if result.Status != "NEEDS_REVIEW" {
		t.Fatalf("status = %s for an unreadable response, want NEEDS_REVIEW", result.Status)
	}

	// It shows up in the queue that needs a human.
	waiting, err := f.fulfilments.ListNeedingAttention(ctx, time.Hour, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found bool
	for _, item := range waiting {
		if item.OrderID == f.orderID {
			found = true
		}
	}
	if !found {
		t.Fatal("an unreadable fulfilment is not in the attention queue")
	}

	// And a human can close it, which a later supplier message must not undo.
	if err := f.service.ResolveManually(ctx, f.orderID, "DELIVERED", "staff-1", "dicek manual ke supplier"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if status, _ := f.status(t); status != "DELIVERED" {
		t.Fatalf("status = %s after manual resolution, want DELIVERED", status)
	}
	if _, err := f.service.ApplyCallback(ctx, f.token, f.orderID, "GAGAL"); err != nil {
		t.Fatalf("late callback: %v", err)
	}
	if status, _ := f.status(t); status != "DELIVERED" {
		t.Fatalf("a late supplier message overwrote a human's decision (status=%s)", status)
	}
}

// A callback token belongs to one supplier. Another's must settle nothing.
func TestCallbackRejectsAnUnknownTokenIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()
	f.service.Open(ctx, f.orderID, f.operatorID)

	if _, err := f.service.ApplyCallback(ctx, "not-a-real-token", f.orderID, `{"status":"OK","sn":"X","harga":1}`); err == nil {
		t.Fatal("an unknown token settled a fulfilment")
	}
	if status, _ := f.status(t); status != "PENDING" {
		t.Fatalf("status = %s after a rejected callback, want PENDING", status)
	}
}
