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
		repository.NewAuditRepository(queries), ledger, repository.NewRefundRepository(pool),
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

// Partial refunds accumulate, and the commission clawback is derived from the
// running total rather than each refund alone. Amounts that do not divide
// evenly are deliberate: rounding each refund down independently would leave
// the agent credited a few rupiah for a sale that no longer exists.
func TestRefundOrderPartialsReverseCommissionExactlyIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()

	for index, amount := range []int64{333_333, 333_333, 333_334} {
		result, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
			OrderId: f.orderID, AmountIdr: amount, Reason: "pembatalan jamaah",
			IdempotencyKey: uuid.NewString(),
		})
		if err != nil {
			t.Fatalf("refund %d: %v", index+1, err)
		}
		if !result.Created {
			t.Fatalf("refund %d reported as a replay", index+1)
		}
	}

	pilgrim, commission := f.balances(t)
	if pilgrim != refundOrderTotal {
		t.Fatalf("pilgrim balance = %d, want the full %d returned", pilgrim, refundOrderTotal)
	}
	// The point of deriving from the total: three floor-rounded partials still
	// land on exactly zero.
	if commission != 0 {
		t.Fatalf("commission balance = %d after a full refund, want 0", commission)
	}
	if status := f.orderStatus(t); status != "REFUNDED" {
		t.Fatalf("order status = %s, want REFUNDED", status)
	}

	// Nothing is left to return.
	if _, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, AmountIdr: 1, IdempotencyKey: uuid.NewString(),
	}); err == nil {
		t.Fatal("a fully refunded order accepted another refund")
	}
}

// A partial refund leaves the sale standing, so the order stays PAID and the
// agent keeps the commission on the part that was not returned.
func TestRefundOrderPartialLeavesOrderPaidIntegration(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()

	result, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, AmountIdr: 250_000, Reason: "potongan", IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if result.Order.Status != "PAID" {
		t.Fatalf("order status = %s after a partial refund, want PAID", result.Order.Status)
	}
	if result.PilgrimBalanceIdr != 250_000 {
		t.Fatalf("reported balance = %d, want 250000", result.PilgrimBalanceIdr)
	}
	pilgrim, commission := f.balances(t)
	if pilgrim != 250_000 || commission != 75_000 {
		t.Fatalf("balances = (%d, %d), want (250000, 75000)", pilgrim, commission)
	}

	// More than what is left cannot be returned.
	if _, err := f.service.RefundOrder(ctx, f.orgID, "user-refund", &hajjv1.RefundOrderRequest{
		OrderId: f.orderID, AmountIdr: 800_000, IdempotencyKey: uuid.NewString(),
	}); err == nil {
		t.Fatal("a refund exceeding the remaining amount was accepted")
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
				OrderId: f.orderID, AmountIdr: 400_000, Reason: "retry", IdempotencyKey: key,
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
	if pilgrim != 400_000 {
		t.Fatalf("pilgrim credited %d across %d replays, want 400000", pilgrim, attempts)
	}
	if commission != 60_000 {
		t.Fatalf("commission balance = %d, want 60000", commission)
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
		OrderId: f.orderID, AmountIdr: 1_000, IdempotencyKey: uuid.NewString(),
	}); err == nil {
		t.Fatal("an unpaid order was refunded")
	}
}
