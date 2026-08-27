package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
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

// The outbound half, against a stub that answers the way a real terminal does.
// Everything up to here was tested from the supplier's side; this is ours.
func TestDispatchSendsAndSettlesIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()

	var seenQuery string
	var seenCount int
	terminal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		seenCount++
		_, _ = w.Write([]byte(`{"status":"OK","sn":"SN-DISPATCH","harga":9.750}`))
	}))
	t.Cleanup(terminal.Close)

	// A plain GET terminal — the shape a good many of these still use.
	if _, err := f.pool.Exec(ctx, `
		UPDATE suppliers SET protocol = 'HTTP_GET', base_url = $2,
			request_template = 'kode={{sku}}&reff={{reference}}&tujuan={{destination}}'
		WHERE id = $1`, f.supplierID, terminal.URL); err != nil {
		t.Fatalf("configure supplier: %v", err)
	}
	f.service.Open(ctx, f.orderID, f.operatorID)

	pending, err := repository.NewSupplierRepository(f.pool).ListPendingDispatch(ctx, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	var mine *repository.PendingDispatch
	for index := range pending {
		if pending[index].OrderID == f.orderID {
			mine = &pending[index]
		}
	}
	if mine == nil {
		t.Fatal("the paid order is not queued for dispatch")
	}

	if err := f.service.Dispatch(ctx, *mine); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// Our own order id goes out as the reference — that is what makes a retry
	// the same purchase to the supplier rather than a second one.
	if !strings.Contains(seenQuery, "reff="+f.orderID) || !strings.Contains(seenQuery, "kode=PULSA10") {
		t.Fatalf("the terminal received %q", seenQuery)
	}

	status, reference := f.status(t)
	if status != "DELIVERED" || reference != "SN-DISPATCH" {
		t.Fatalf("status=%s reference=%s, want DELIVERED and SN-DISPATCH", status, reference)
	}

	// A second pass must not send again: the claim is the lock.
	if err := f.service.Dispatch(ctx, *mine); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if seenCount != 1 {
		t.Fatalf("the terminal was called %d times, want 1", seenCount)
	}

	// The exchange is on the record, with the request beside the response.
	var request, response string
	if err := f.pool.QueryRow(ctx,
		`SELECT request_body, response_body FROM supplier_request_logs WHERE order_id = $1 AND direction = 'REQUEST'`,
		f.orderID).Scan(&request, &response); err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(response, "SN-DISPATCH") || !strings.Contains(request, "kode=PULSA10") {
		t.Fatalf("log did not capture the exchange: %q / %q", request, response)
	}
}

// A supplier that never answers must not look like one that refused. The
// difference decides whether a jamaah gets refunded for something already sent.
func TestDispatchWithNoAnswerWaitsForAPersonIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()

	// Closed immediately, so the connection is refused rather than hanging.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	if _, err := f.pool.Exec(ctx, `
		UPDATE suppliers SET protocol = 'HTTP_GET', base_url = $2, request_template = 'ref={{reference}}'
		WHERE id = $1`, f.supplierID, deadURL); err != nil {
		t.Fatalf("configure supplier: %v", err)
	}
	f.service.Open(ctx, f.orderID, f.operatorID)

	pending, err := repository.NewSupplierRepository(f.pool).ListPendingDispatch(ctx, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	for _, item := range pending {
		if item.OrderID != f.orderID {
			continue
		}
		if err := f.service.Dispatch(ctx, item); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
	}

	status, _ := f.status(t)
	if status != "NEEDS_REVIEW" {
		t.Fatalf("status = %s when the supplier never answered, want NEEDS_REVIEW — not FAILED", status)
	}
	// And the attempt was counted, so a lost request never looks like one that
	// was never sent.
	var attempts int
	if err := f.pool.QueryRow(ctx, `SELECT attempts FROM order_fulfilments WHERE order_id = $1`, f.orderID).Scan(&attempts); err != nil {
		t.Fatalf("read attempts: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

// A supplier address pointing inside our own network is refused before anything
// is sent, and lands in front of a person rather than being retried forever.
func TestDispatchRefusesAnInternalSupplierAddressIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()

	if _, err := f.pool.Exec(ctx, `
		UPDATE suppliers SET protocol = 'HTTP_GET', base_url = 'http://169.254.169.254/latest/meta-data/',
			request_template = 'ref={{reference}}'
		WHERE id = $1`, f.supplierID); err != nil {
		t.Fatalf("configure supplier: %v", err)
	}
	f.service.Open(ctx, f.orderID, f.operatorID)

	pending, err := repository.NewSupplierRepository(f.pool).ListPendingDispatch(ctx, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	for _, item := range pending {
		if item.OrderID == f.orderID {
			if err := f.service.Dispatch(ctx, item); err != nil {
				t.Fatalf("dispatch: %v", err)
			}
		}
	}

	var status, reason string
	if err := f.pool.QueryRow(ctx, `SELECT status, last_error FROM order_fulfilments WHERE order_id = $1`,
		f.orderID).Scan(&status, &reason); err != nil {
		t.Fatalf("read fulfilment: %v", err)
	}
	if status != "NEEDS_REVIEW" {
		t.Fatalf("status = %s, want NEEDS_REVIEW", status)
	}
	if !strings.Contains(reason, "jaringan internal") {
		t.Fatalf("reason = %q, want it to name the internal address", reason)
	}
}

// recordingQueue stands in for the real one so the test can see whether the
// fast path fired, without needing Redis.
type recordingQueue struct {
	mu       sync.Mutex
	enqueued []string
	fail     error
}

func (q *recordingQueue) EnqueueDispatch(_ context.Context, orderID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.fail != nil {
		return q.fail
	}
	q.enqueued = append(q.enqueued, orderID)
	return nil
}

// A digital product has to go out in seconds, not at the next sweep. Opening a
// fulfilment therefore hands it straight to the worker.
func TestOpeningAFulfilmentSendsItImmediatelyIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()
	sender := &recordingQueue{}
	f.service.AttachQueue(sender)

	f.service.Open(ctx, f.orderID, f.operatorID)

	if len(sender.enqueued) != 1 || sender.enqueued[0] != f.orderID {
		t.Fatalf("enqueued %v, want exactly this order — otherwise it waits for the sweep", sender.enqueued)
	}
}

// A queue that is briefly unreachable must never cost the payment. The money
// has settled and the fulfilment row already records the debt; the sweep is
// underneath for exactly this.
func TestAFailedEnqueueStillLeavesTheFulfilmentRecordedIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()
	f.service.AttachQueue(&recordingQueue{fail: errors.New("redis unreachable")})

	f.service.Open(ctx, f.orderID, f.operatorID)

	status, _ := f.status(t)
	if status != "PENDING" {
		t.Fatalf("status = %s after a failed enqueue, want PENDING so the sweep picks it up", status)
	}
	// And the sweep does find it.
	pending, err := repository.NewSupplierRepository(f.pool).ListPendingDispatch(ctx, 50)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	var found bool
	for _, item := range pending {
		if item.OrderID == f.orderID {
			found = true
		}
	}
	if !found {
		t.Fatal("an order whose enqueue failed is invisible to the sweep")
	}
}

// The fast path and the sweep can reach the same order. Whichever arrives
// second must find nothing to do rather than sending twice.
func TestFastPathAndSweepCannotBothSendIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()

	var calls int32
	terminal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"status":"OK","sn":"SN-RACE","harga":9.000}`))
	}))
	t.Cleanup(terminal.Close)
	if _, err := f.pool.Exec(ctx, `
		UPDATE suppliers SET protocol = 'HTTP_GET', base_url = $2, request_template = 'ref={{reference}}'
		WHERE id = $1`, f.supplierID, terminal.URL); err != nil {
		t.Fatalf("configure supplier: %v", err)
	}
	f.service.Open(ctx, f.orderID, f.operatorID)

	suppliers := repository.NewSupplierRepository(f.pool)
	pending, err := suppliers.PendingDispatchFor(ctx, f.orderID)
	if err != nil {
		t.Fatalf("resolve pending: %v", err)
	}

	// Both paths, at once.
	const racers = 5
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := f.service.Dispatch(ctx, pending); err != nil {
				t.Errorf("dispatch: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("the supplier was called %d times from %d concurrent dispatches, want 1", got, racers)
	}
	if status, _ := f.status(t); status != "DELIVERED" {
		t.Fatalf("status = %s, want DELIVERED", status)
	}
}

// Refunding something the supplier already delivered is a straight loss: we
// paid them and gave the money back too.
func TestRefundIsRefusedOnceDeliveredIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()
	queries := db.New(f.pool)
	orders := NewOrderService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		repository.NewProductRepository(queries, f.pool), repository.NewOrderRepository(queries),
		repository.NewAuditRepository(queries), repository.NewLedgerRepository(f.pool),
		repository.NewRefundRepository(f.pool), repository.NewAgentRepository(queries),
		f.pool, nil, "http://localhost:3000")
	orders.AttachFulfilment(f.service, f.fulfilments)

	var orgID string
	if err := f.pool.QueryRow(ctx, `SELECT better_auth_org_id FROM operators WHERE id = $1`, f.operatorID).Scan(&orgID); err != nil {
		t.Fatalf("read org: %v", err)
	}
	refund := func() error {
		_, err := orders.RefundOrder(ctx, orgID, "staff-1", &hajjv1.RefundOrderRequest{
			OrderId: f.orderID, Reason: "uji", IdempotencyKey: uuid.NewString(),
		})
		return err
	}

	f.service.Open(ctx, f.orderID, f.operatorID)

	// Still out at the supplier: the answer is not known yet, so refunding now
	// could easily be refunding something on its way.
	if _, err := f.fulfilments.Claim(ctx, f.orderID); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := refund(); err == nil {
		t.Fatal("refunded while the supplier was still working on it")
	}

	// Delivered: refused outright.
	if _, err := f.service.ApplyCallback(ctx, f.token, f.orderID, `{"status":"OK","sn":"SN-9","harga":9.000}`); err != nil {
		t.Fatalf("callback: %v", err)
	}
	err := refund()
	if err == nil {
		t.Fatal("refunded a product the supplier had already delivered")
	}
	if !strings.Contains(err.Error(), "sudah dikirim") {
		t.Fatalf("error did not explain why: %v", err)
	}

	// The way out is to correct the delivery record, which leaves a trace of
	// somebody deciding that — a bypass flag would leave none.
	if err := f.service.ResolveManually(ctx, f.orderID, "FAILED", "staff-1", "dicek: tidak terkirim"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := refund(); err != nil {
		t.Fatalf("refund refused after the delivery was corrected: %v", err)
	}
}

// Marking an order paid and opening its fulfilment are two writes, not one
// transaction. A process dying between them leaves a paid order that no
// dispatch path can see, because every one of them starts from the fulfilment
// row.
func TestSweepRecoversAPaidOrderWithNoFulfilmentIntegration(t *testing.T) {
	f := newFulfilmentFixture(t)
	ctx := context.Background()

	// The order is PAID from the fixture and no fulfilment was ever opened —
	// exactly the state a crash between the two writes leaves behind.
	var rows int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM order_fulfilments WHERE order_id = $1`, f.orderID).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Fatalf("the fixture already has a fulfilment (%d)", rows)
	}

	opened, err := f.fulfilments.OpenMissing(ctx)
	if err != nil {
		t.Fatalf("open missing: %v", err)
	}
	if opened < 1 {
		t.Fatal("a paid order owing a delivery was not recovered")
	}
	status, _ := f.status(t)
	if status != "PENDING" {
		t.Fatalf("recovered fulfilment status = %s, want PENDING so it gets sent", status)
	}

	// Running again must not create a second one.
	if _, err := f.fulfilments.OpenMissing(ctx); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM order_fulfilments WHERE order_id = $1`, f.orderID).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Fatalf("%d fulfilments after two recovery passes, want 1", rows)
	}
}
