package service

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type refundFixture struct {
	pool     *pgxpool.Pool
	service  *OrderService
	ledger   *repository.LedgerRepository
	orgID    string
	orderID  string
	agentID  string
	pilgrimD string
}

const (
	refundOrderTotal = int64(1_000_000)
	refundCommission = int64(100_000)
)

// A paid order with an agent commission, set up the way the payment path
// leaves it: the order row plus the EARNED entry in the commission ledger.
func newRefundFixture(t *testing.T) *refundFixture {
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

	operatorID := uuid.NewString()
	orgID := "refund-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Refund Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.com", "refund-"+operatorID[:8]); err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := pool.Begin(context.Background())
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Rollback(context.Background()) }()
		if _, err := cleanup.Exec(context.Background(), `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
			return
		}
		// order_refunds references orders with RESTRICT, so it must go before
		// the operator cascade can reach the orders themselves.
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM order_refunds WHERE operator_id = $1`, operatorID); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		_ = cleanup.Commit(context.Background())
	})

	agentID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, operator_id, name) VALUES ($1,$2,'Agen Refund')`, agentID, operatorID); err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	seasonID, pilgrimID, productID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Refund','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID); err != nil {
		t.Fatalf("insert season: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,'Jamaah Refund','P-REF','ID','1990-01-01'::timestamptz,'MALE')`, pilgrimID, seasonID, operatorID); err != nil {
		t.Fatalf("insert pilgrim: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO products (id, operator_id, season_id, name, price_idr) VALUES ($1,$2,$3,'Produk Refund',$4)`, productID, operatorID, seasonID, refundOrderTotal); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	var orderID string
	if err := pool.QueryRow(ctx, `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, agent_id, unit_price_idr, total_price_idr, agent_commission_idr, status, paid_at)
		VALUES ($1,$2,$3,$4,$5,$6,$6,$7,'PAID',NOW()) RETURNING id::text`,
		operatorID, seasonID, pilgrimID, productID, agentID, refundOrderTotal, refundCommission).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	ledger := repository.NewLedgerRepository(pool)
	if err := ledger.AppendCommission(ctx, repository.CommissionEntry{
		OperatorID: operatorID, AgentID: agentID, AmountIDR: refundCommission, Kind: "EARNED",
		OrderID: orderID, IdempotencyKey: "order-earned-" + orderID,
	}); err != nil {
		t.Fatalf("append commission: %v", err)
	}

	queries := db.New(pool)
	service := NewOrderService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		repository.NewProductRepository(queries, pool), repository.NewOrderRepository(queries),
		repository.NewAuditRepository(queries), ledger, repository.NewRefundRepository(pool), repository.NewAgentRepository(queries),
		pool, nil, "http://localhost:3000")

	return &refundFixture{pool: pool, service: service, ledger: ledger, orgID: orgID, orderID: orderID, agentID: agentID, pilgrimD: pilgrimID}
}

func (f *refundFixture) balances(t *testing.T) (pilgrim, commission int64) {
	t.Helper()
	ctx := context.Background()
	pilgrim, err := f.ledger.PilgrimBalance(ctx, f.pilgrimD)
	if err != nil {
		t.Fatalf("pilgrim balance: %v", err)
	}
	commission, err = f.ledger.CommissionBalance(ctx, f.agentID)
	if err != nil {
		t.Fatalf("commission balance: %v", err)
	}
	return pilgrim, commission
}

func (f *refundFixture) orderStatus(t *testing.T) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM orders WHERE id = $1`, f.orderID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

// A refund returns the whole transaction: the pilgrim gets back what they
// paid, the agent's commission is reversed in full, and the order is REFUNDED.
func TestRefundOrderReturnsTheWholeTransactionIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()

	result, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, Reason: "pembatalan jamaah", IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if !result.Created {
		t.Fatal("the first refund reported as a replay")
	}
	if result.Refund.AmountIdr != refundOrderTotal {
		t.Fatalf("refunded %d, want the full %d", result.Refund.AmountIdr, refundOrderTotal)
	}
	if result.Refund.CommissionReversedIdr != refundCommission {
		t.Fatalf("reversed %d commission, want the full %d", result.Refund.CommissionReversedIdr, refundCommission)
	}
	if result.Order.Status != "REFUNDED" {
		t.Fatalf("order status = %s, want REFUNDED", result.Order.Status)
	}

	pilgrim, commission := f.balances(t)
	if pilgrim != refundOrderTotal {
		t.Fatalf("pilgrim balance = %d, want %d", pilgrim, refundOrderTotal)
	}
	if commission != 0 {
		t.Fatalf("commission balance = %d after a refund, want 0", commission)
	}

	// Nothing is left to return, and the order cannot be refunded twice.
	if _, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, IdempotencyKey: uuid.NewString(),
	}); err == nil {
		t.Fatal("a refunded order was refunded again")
	}
}

// The same idempotency key always refers to the same refund. A double-clicked
// button or a retried call must settle that one refund, never issue a second.
func TestRefundOrderIsIdempotentUnderConcurrencyIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()

	const attempts = 6
	key := uuid.NewString()
	var wg sync.WaitGroup
	createdCount := make([]bool, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
				OrderId: f.orderID, Reason: "retry", IdempotencyKey: key,
			})
			if err != nil {
				t.Errorf("attempt %d: %v", index, err)
				return
			}
			createdCount[index] = result.Created
		}(i)
	}
	wg.Wait()

	created := 0
	for _, wasCreated := range createdCount {
		if wasCreated {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("%d of %d attempts reported creating the refund, want exactly 1", created, attempts)
	}

	var rows int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM order_refunds WHERE order_id = $1`, f.orderID).Scan(&rows); err != nil {
		t.Fatalf("count refunds: %v", err)
	}
	if rows != 1 {
		t.Fatalf("%d refund rows recorded, want 1", rows)
	}
	pilgrim, commission := f.balances(t)
	if pilgrim != refundOrderTotal {
		t.Fatalf("pilgrim credited %d across %d replays, want %d", pilgrim, attempts, refundOrderTotal)
	}
	if commission != 0 {
		t.Fatalf("commission balance = %d, want 0", commission)
	}
}

// Only money that actually arrived can be sent back.
func TestRefundOrderRejectsUnpaidOrderIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE orders SET status = 'PENDING', paid_at = NULL WHERE id = $1`, f.orderID); err != nil {
		t.Fatalf("unpay order: %v", err)
	}
	if _, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, IdempotencyKey: uuid.NewString(),
	}); err == nil {
		t.Fatal("an unpaid order was refunded")
	}
}

// The bug this guards against was not a report being slightly off. A pilgrim's
// "total paid" is multiplied by the cancellation policy's refund percentage,
// so counting an already-refunded amount as still paid made the operator
// return the same money a second time.
func TestPilgrimPaidTotalIsNetOfRefundsIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()
	cancellations := repository.NewCancellationRepository(f.pool, db.New(f.pool))

	paid, err := cancellations.GetPaidTotal(ctx, f.pilgrimD)
	if err != nil {
		t.Fatalf("paid total: %v", err)
	}
	if paid != refundOrderTotal {
		t.Fatalf("paid total = %d before any refund, want %d", paid, refundOrderTotal)
	}

	if _, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, Reason: "pembatalan", IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatalf("refund: %v", err)
	}

	if paid, err = cancellations.GetPaidTotal(ctx, f.pilgrimD); err != nil || paid != 0 {
		t.Fatalf("paid total = %d (%v) after a refund, want 0", paid, err)
	}
}

// The service cannot express a partial refund, because the request carries no
// amount. This asserts the rule underneath that: a path which never reaches
// the service still cannot write one.
func TestDatabaseRejectsPartialAndRepeatedRefundsIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()
	var operatorID string
	if err := f.pool.QueryRow(ctx, `SELECT operator_id::text FROM orders WHERE id = $1`, f.orderID).Scan(&operatorID); err != nil {
		t.Fatalf("read operator: %v", err)
	}

	insert := `INSERT INTO order_refunds (operator_id, order_id, amount_idr, reason) VALUES ($1, $2, $3, 'langsung')`
	if _, err := f.pool.Exec(ctx, insert, operatorID, f.orderID, refundOrderTotal/2); err == nil {
		t.Fatal("a partial refund was written straight to the table")
	}
	if _, err := f.pool.Exec(ctx, insert, operatorID, f.orderID, refundOrderTotal+1); err == nil {
		t.Fatal("a refund larger than the order was accepted")
	}

	// The whole amount is accepted, and only once.
	if _, err := f.pool.Exec(ctx, insert, operatorID, f.orderID, refundOrderTotal); err != nil {
		t.Fatalf("a full refund was rejected: %v", err)
	}
	if _, err := f.pool.Exec(ctx, insert, operatorID, f.orderID, refundOrderTotal); err == nil {
		t.Fatal("the same order was refunded twice")
	}

	// An order nobody paid cannot be refunded at all.
	other := newRefundFixture(t)
	if _, err := other.pool.Exec(ctx, `UPDATE orders SET status = 'PENDING' WHERE id = $1`, other.orderID); err != nil {
		t.Fatalf("unpay: %v", err)
	}
	var otherOperator string
	if err := other.pool.QueryRow(ctx, `SELECT operator_id::text FROM orders WHERE id = $1`, other.orderID).Scan(&otherOperator); err != nil {
		t.Fatalf("read operator: %v", err)
	}
	if _, err := other.pool.Exec(ctx, insert, otherOperator, other.orderID, refundOrderTotal); err == nil {
		t.Fatal("an unpaid order was refunded directly")
	}
}

// A paid order whose commission never reached the ledger — the shape left
// behind by an order paid between the migration and the API restart.
func TestReconcileCreditsCommissionTheWritePathMissedIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()

	// Remove the EARNED entry to recreate the gap. Ledger rows refuse deletion
	// without the teardown flag, which is the point of them.
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
		t.Fatalf("purge flag: %v", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM agent_commission_entries WHERE order_id = $1`, f.orderID); err != nil {
		t.Fatalf("delete entry: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, commission := f.balances(t); commission != 0 {
		t.Fatalf("commission = %d, the gap was not created", commission)
	}

	if _, err := f.ledger.ReconcileEarnedCommission(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, commission := f.balances(t); commission != refundCommission {
		t.Fatalf("commission = %d after reconciliation, want %d", commission, refundCommission)
	}

	// Running again must not credit a second time — the sweep uses the same
	// idempotency key the payment path does.
	if _, err := f.ledger.ReconcileEarnedCommission(ctx); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if _, commission := f.balances(t); commission != refundCommission {
		t.Fatalf("commission = %d after a second sweep, want %d", commission, refundCommission)
	}

	// A reversal is not a discrepancy: the sweep must leave a refunded order's
	// clawback alone rather than "restoring" the earning.
	if _, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, Reason: "penuh", IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if _, err := f.ledger.ReconcileEarnedCommission(ctx); err != nil {
		t.Fatalf("reconcile after refund: %v", err)
	}
	if _, commission := f.balances(t); commission != 0 {
		t.Fatalf("commission = %d after reconciling a refunded order, want 0", commission)
	}
}

// A retry that arrives after the original refund has fully settled — the
// operator never saw the first response and pressed the button again.
//
// This is the case a status precondition gets wrong: by then the order is
// REFUNDED, so a naive check rejects the retry with "only paid orders can be
// refunded", and the caller concludes the refund failed when it succeeded.
func TestRefundOrderReplayAfterSettlementReturnsTheOriginalIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()
	key := uuid.NewString()

	first, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, Reason: "pembatalan", IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("first refund: %v", err)
	}

	replay, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, Reason: "pembatalan", IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("replay rejected: %v", err)
	}
	if replay.Created {
		t.Fatal("the replay reported creating a second refund")
	}
	if replay.Refund.Id != first.Refund.Id {
		t.Fatalf("replay returned refund %s, want the original %s", replay.Refund.Id, first.Refund.Id)
	}
	if replay.Refund.AmountIdr != first.Refund.AmountIdr || replay.PilgrimBalanceIdr != first.PilgrimBalanceIdr {
		t.Fatal("the replay reported different amounts from the original")
	}

	// And it stayed one refund, crediting the pilgrim once.
	var rows int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM order_refunds WHERE order_id = $1`, f.orderID).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Fatalf("%d refund rows, want 1", rows)
	}
	if pilgrim, _ := f.balances(t); pilgrim != refundOrderTotal {
		t.Fatalf("pilgrim balance = %d, want %d", pilgrim, refundOrderTotal)
	}

	// A different key on a settled order is a new request, not a replay, and
	// must still be refused.
	if _, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, IdempotencyKey: uuid.NewString(),
	}); err == nil {
		t.Fatal("a settled order accepted a second refund under a new key")
	}
}
