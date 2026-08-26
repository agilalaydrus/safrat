package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type referralFixture struct {
	pool       *pgxpool.Pool
	orders     *OrderService
	operatorID string
	orgID      string
	seasonID   string
	productID  string
	referrer   string
	seller     string
	pilgrimID  string
}

const referralPrice = int64(4_000_000)

// A jamaah referred by one agent, a second agent who referred nobody, and a
// product with a 15% agent margin.
func newReferralFixture(t *testing.T) *referralFixture {
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

	operatorID, orgID := uuid.NewString(), "referral-"+uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture %q: %v", query, err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Referral Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "ref-"+operatorID[:8])
	t.Cleanup(func() {
		cleanup, err := pool.Begin(context.Background())
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Rollback(context.Background()) }()
		if _, err := cleanup.Exec(context.Background(), `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		_ = cleanup.Commit(context.Background())
	})

	referrer, seller := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO agents (id, operator_id, name) VALUES ($1,$2,'Agen Pereferral')`, referrer, operatorID)
	exec(`INSERT INTO agents (id, operator_id, name) VALUES ($1,$2,'Agen Penjual')`, seller, operatorID)

	seasonID, productID, pilgrimID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, price_idr, agent_margin_pct) VALUES ($1,$2,$3,'Paket Uji',$4,0.15)`,
		productID, operatorID, seasonID, referralPrice)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender, agent_id)
	      VALUES ($1,$2,$3,'Jamaah Referral','P-REF','ID','1990-01-01'::timestamptz,'MALE',$4)`,
		pilgrimID, seasonID, operatorID, referrer)

	// A stub Xendit, so the invoice call is actually exercised rather than
	// short-circuited by an unconfigured client — which would let an order
	// path fail silently and the assertions below pass on an empty result.
	invoices := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inv-` + uuid.NewString() + `","invoice_url":"https://stub.invalid/pay"}`))
	}))
	t.Cleanup(invoices.Close)

	queries := db.New(pool)
	orders := NewOrderService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		repository.NewProductRepository(queries, pool), repository.NewOrderRepository(queries),
		repository.NewAuditRepository(queries), repository.NewLedgerRepository(pool),
		repository.NewRefundRepository(pool), repository.NewAgentRepository(queries),
		pool, payment.NewClientWithEndpoint("test-key", invoices.URL), "http://localhost:3000")

	return &referralFixture{pool: pool, orders: orders, operatorID: operatorID, orgID: orgID,
		seasonID: seasonID, productID: productID, referrer: referrer, seller: seller, pilgrimID: pilgrimID}
}

func (f *referralFixture) orderRow(t *testing.T, orderID string) (agentID string, placedBy *string, commission int64) {
	t.Helper()
	var agent *string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT agent_id::text, placed_by_agent_id::text, agent_commission_idr FROM orders WHERE id = $1`,
		orderID).Scan(&agent, &placedBy, &commission); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if agent != nil {
		agentID = *agent
	}
	return agentID, placedBy, commission
}

// The referral earns the commission however the jamaah reaches checkout. This
// used to be hard-coded to zero on the jamaah's own checkout, so a referral
// produced nothing the moment they bought for themselves.
func TestReferralEarnsOnManualOrderIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()

	response, err := f.orders.CreateManualOrder(ctx, f.orgID, &hajjv1.CreateManualOrderRequest{
		PilgrimId: f.pilgrimID, ProductId: f.productID, Quantity: 1,
		PaymentMethod:  hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_CASH,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("manual order: %v", err)
	}

	agentID, _, commission := f.orderRow(t, response.Order.Id)
	if agentID != f.referrer {
		t.Fatalf("order credited agent %s, want the referrer %s", agentID, f.referrer)
	}
	// 15% of 4,000,000.
	if commission != 600_000 {
		t.Fatalf("commission = %d, want 600000", commission)
	}

	// Paid immediately by the CASH path, so the ledger should already carry it.
	balance, err := repository.NewLedgerRepository(f.pool).CommissionBalance(ctx, f.referrer)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if balance != 600_000 {
		t.Fatalf("referrer's ledger balance = %d, want 600000", balance)
	}
}

// Selling is open — any agent may transact for any jamaah — but the commission
// follows the referral, so selling into somebody else's referral credits that
// referrer and not the seller.
func TestSellingToAnotherAgentsReferralCreditsTheReferrerIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()

	// The seller acts as themselves, resolved from their own linked identity.
	sellerUserID := "seller-" + uuid.NewString()
	if _, err := f.pool.Exec(ctx, `INSERT INTO "user" (id, name, email, "emailVerified") VALUES ($1,'Agen Penjual',$2,true)`,
		sellerUserID, sellerUserID+"@example.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = f.pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, sellerUserID) })
	if _, err := f.pool.Exec(ctx, `UPDATE agents SET linked_user_id = $1 WHERE id = $2`, sellerUserID, f.seller); err != nil {
		t.Fatalf("link agent: %v", err)
	}

	response, err := f.orders.CreateOrderForPilgrim(ctx, f.orgID, sellerUserID, &hajjv1.CreateOrderForPilgrimRequest{
		PilgrimId: f.pilgrimID, ProductId: f.productID, Quantity: 1, IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("agent order: %v", err)
	}

	agentID, placedBy, commission := f.orderRow(t, response.Order.Id)
	if agentID != f.referrer {
		t.Fatalf("commission went to %s, want the referrer %s — a seller must not take another agent's referral", agentID, f.referrer)
	}
	if placedBy == nil || *placedBy != f.seller {
		t.Fatalf("placed_by = %v, want the seller %s", placedBy, f.seller)
	}
	// 15% of 4,000,000, and it belongs to the referrer.
	if commission != 600_000 {
		t.Fatalf("commission = %d, want 600000", commission)
	}
	if response.CheckoutUrl == "" {
		t.Fatal("no checkout link was returned")
	}

	// The seller earned nothing from selling into someone else's referral.
	ledger := repository.NewLedgerRepository(f.pool)
	if balance, err := ledger.CommissionBalance(ctx, f.seller); err != nil || balance != 0 {
		t.Fatalf("seller's balance = %d (%v), want 0", balance, err)
	}
}

// A double-tapped checkout used to make two orders and two invoices, either of
// which the jamaah could pay.
func TestManualOrderIsIdempotentUnderConcurrencyIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()
	key := uuid.NewString()

	const attempts = 6
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.orders.CreateManualOrder(ctx, f.orgID, &hajjv1.CreateManualOrderRequest{
				PilgrimId: f.pilgrimID, ProductId: f.productID, Quantity: 1,
				PaymentMethod:  hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_CASH,
				IdempotencyKey: key,
			}); err != nil {
				t.Errorf("attempt: %v", err)
			}
		}()
	}
	wg.Wait()

	var orders int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE pilgrim_id = $1`, f.pilgrimID).Scan(&orders); err != nil {
		t.Fatalf("count: %v", err)
	}
	if orders != 1 {
		t.Fatalf("%d orders created from %d replays of one key, want 1", orders, attempts)
	}
	// And the commission was credited exactly once.
	balance, err := repository.NewLedgerRepository(f.pool).CommissionBalance(ctx, f.referrer)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if balance != 600_000 {
		t.Fatalf("commission balance = %d after %d replays, want 600000", balance, attempts)
	}
}
