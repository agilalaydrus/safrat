package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
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
	exec(`INSERT INTO products (id, operator_id, season_id, name, price_idr, agent_margin_bps) VALUES ($1,$2,$3,'Paket Uji',$4,1500)`,
		productID, operatorID, seasonID, referralPrice)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender, agent_id)
	      VALUES ($1,$2,$3,'Jamaah Referral','P-REF','ID','1990-01-01'::timestamptz,'MALE',$4)`,
		pilgrimID, seasonID, operatorID, referrer)

	// A stub Xendit, so the invoice call is actually exercised rather than
	// short-circuited by an unconfigured client — which would let an order
	// path fail silently and the assertions below pass on an empty result.
	invoices := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// A fetch: report the invoice as paid in full, which is what the
			// settlement path asks the gateway rather than the caller.
			id := strings.TrimPrefix(r.URL.Path, "/")
			_, _ = w.Write([]byte(`{"id":"` + id + `","status":"PAID","amount":` +
				strconv.FormatInt(referralPrice, 10) + `,"paid_amount":` + strconv.FormatInt(referralPrice, 10) + `}`))
			return
		}
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

// The jamaah's own checkout code, so tests can drive the self-service lane.
func (f *referralFixture) accessCode(t *testing.T) string {
	t.Helper()
	var code string
	if err := f.pool.QueryRow(context.Background(), `SELECT app_access_code FROM pilgrims WHERE id = $1`, f.pilgrimID).Scan(&code); err != nil {
		t.Fatalf("read access code: %v", err)
	}
	return code
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

// A pending transaction already counts: the referrer's commission is
// recognised when the transaction is placed, not when it settles. What it
// cannot do is be withdrawn — that would advance money for a transaction that
// may still fail.
func TestPendingCommissionIsRecognisedButNotPayableIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()
	ledger := repository.NewLedgerRepository(f.pool)
	agents := repository.NewAgentRepository(db.New(f.pool))

	response, err := f.orders.CreateOrder(ctx, &hajjv1.CreateOrderRequest{
		AppAccessCode: f.accessCode(t), ProductId: f.productID, Quantity: 1,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if response.Order.Status != "PENDING" {
		t.Fatalf("order status = %s, want PENDING", response.Order.Status)
	}

	// Recognised straight away.
	if balance, err := ledger.CommissionBalance(ctx, f.referrer); err != nil || balance != 600_000 {
		t.Fatalf("recognised commission = %d (%v), want 600000 while pending", balance, err)
	}
	summary, err := agents.GetPayoutSummary(ctx, f.operatorID, f.referrer)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalCommissionIDR != 600_000 {
		t.Fatalf("total = %d, want 600000 — a pending transaction counts", summary.TotalCommissionIDR)
	}
	if summary.PendingCommissionIDR != 600_000 {
		t.Fatalf("pending = %d, want 600000", summary.PendingCommissionIDR)
	}
	// ...but not a rupiah of it is withdrawable yet.
	if summary.SettledCommissionIDR != 0 || summary.OutstandingIDR != 0 {
		t.Fatalf("settled=%d outstanding=%d, want 0 and 0 — pending commission is not payable",
			summary.SettledCommissionIDR, summary.OutstandingIDR)
	}

	// Settling the transaction moves it from pending to payable, without a
	// second entry: the ledger did not change, only the transaction behind it.
	var invoiceID string
	if err := f.pool.QueryRow(ctx, `SELECT xendit_invoice_id FROM orders WHERE id = $1`, response.Order.Id).Scan(&invoiceID); err != nil {
		t.Fatalf("read invoice id: %v", err)
	}
	if err := f.orders.SettlePayment(ctx, invoiceID, referralPrice); err != nil {
		t.Fatalf("settle: %v", err)
	}
	summary, err = agents.GetPayoutSummary(ctx, f.operatorID, f.referrer)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.SettledCommissionIDR != 600_000 || summary.OutstandingIDR != 600_000 {
		t.Fatalf("settled=%d outstanding=%d after payment, want 600000 each",
			summary.SettledCommissionIDR, summary.OutstandingIDR)
	}
	if summary.PendingCommissionIDR != 0 {
		t.Fatalf("pending = %d after payment, want 0", summary.PendingCommissionIDR)
	}
	var entries int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM agent_commission_entries WHERE order_id = $1`, response.Order.Id).Scan(&entries); err != nil {
		t.Fatalf("count: %v", err)
	}
	if entries != 1 {
		t.Fatalf("%d ledger entries, want 1 — settling is a change of transaction state, not a new earning", entries)
	}
}

// A transaction that will never complete gives the commission back.
func TestFailedTransactionReversesRecognisedCommissionIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()
	ledger := repository.NewLedgerRepository(f.pool)

	response, err := f.orders.CreateOrder(ctx, &hajjv1.CreateOrderRequest{
		AppAccessCode: f.accessCode(t), ProductId: f.productID, Quantity: 1,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if balance, err := ledger.CommissionBalance(ctx, f.referrer); err != nil || balance != 600_000 {
		t.Fatalf("recognised = %d (%v), want 600000", balance, err)
	}

	var invoiceID string
	if err := f.pool.QueryRow(ctx, `SELECT xendit_invoice_id FROM orders WHERE id = $1`, response.Order.Id).Scan(&invoiceID); err != nil {
		t.Fatalf("read invoice id: %v", err)
	}
	if err := f.orders.MarkStatusByInvoiceID(ctx, invoiceID, "EXPIRED"); err != nil {
		t.Fatalf("expire: %v", err)
	}

	if balance, err := ledger.CommissionBalance(ctx, f.referrer); err != nil || balance != 0 {
		t.Fatalf("commission = %d (%v) after expiry, want 0", balance, err)
	}
	// The earning is still on the record, cancelled by a reversal rather than
	// erased.
	var entries int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM agent_commission_entries WHERE order_id = $1`, response.Order.Id).Scan(&entries); err != nil {
		t.Fatalf("count: %v", err)
	}
	if entries != 2 {
		t.Fatalf("%d ledger entries, want 2 — the earning and its reversal", entries)
	}
}

// Commission counts only when the amount paid matches the price. Until this
// check existed the webhook settled on the gateway's word that *something* was
// paid, and revenue, commission and the jamaah's history all followed from it.
func TestUnderpaymentIsHeldRatherThanSettledIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()
	agents := repository.NewAgentRepository(db.New(f.pool))

	response, err := f.orders.CreateOrder(ctx, &hajjv1.CreateOrderRequest{
		AppAccessCode: f.accessCode(t), ProductId: f.productID, Quantity: 1,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	var invoiceID string
	if err := f.pool.QueryRow(ctx, `SELECT xendit_invoice_id FROM orders WHERE id = $1`, response.Order.Id).Scan(&invoiceID); err != nil {
		t.Fatalf("read invoice id: %v", err)
	}

	// Paid, but a rupiah short of the price.
	if err := f.orders.SettlePayment(ctx, invoiceID, referralPrice-1); err != nil {
		t.Fatalf("settle: %v", err)
	}

	var status, reason string
	var paidAmount *int64
	if err := f.pool.QueryRow(ctx, `SELECT status, held_reason, paid_amount_idr FROM orders WHERE id = $1`,
		response.Order.Id).Scan(&status, &reason, &paidAmount); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if status != "HELD" {
		t.Fatalf("order status = %s after underpayment, want HELD", status)
	}
	if reason == "" {
		t.Fatal("a held order carries no reason")
	}
	// The evidence is kept: what arrived, not just what was owed.
	if paidAmount == nil || *paidAmount != referralPrice-1 {
		t.Fatalf("recorded paid amount = %v, want %d", paidAmount, referralPrice-1)
	}

	summary, err := agents.GetPayoutSummary(ctx, f.operatorID, f.referrer)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	// Still counted — it neither failed nor was refunded — but not payable.
	if summary.TotalCommissionIDR != 600_000 || summary.PendingCommissionIDR != 600_000 {
		t.Fatalf("total=%d pending=%d, want 600000 each — a held transaction still counts",
			summary.TotalCommissionIDR, summary.PendingCommissionIDR)
	}
	if summary.SettledCommissionIDR != 0 || summary.OutstandingIDR != 0 {
		t.Fatalf("settled=%d outstanding=%d, want 0 — an unverified payment must not become payable",
			summary.SettledCommissionIDR, summary.OutstandingIDR)
	}

	// It is not revenue either.
	var netPaid int64
	if err := f.pool.QueryRow(ctx, `SELECT net_paid_idr FROM order_payments WHERE order_id = $1`, response.Order.Id).Scan(&netPaid); err != nil {
		t.Fatalf("read net paid: %v", err)
	}
	if netPaid != 0 {
		t.Fatalf("net paid = %d for a held order, want 0", netPaid)
	}

	// A redelivered notification must not settle it either.
	if err := f.orders.SettlePayment(ctx, invoiceID, referralPrice); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, response.Order.Id).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "HELD" {
		t.Fatalf("a held order was settled by a later notification (status=%s)", status)
	}
}

// A gateway delivery that reports no amount cannot be treated as a match.
func TestUnreportedAmountIsHeldIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()

	response, err := f.orders.CreateOrder(ctx, &hajjv1.CreateOrderRequest{
		AppAccessCode: f.accessCode(t), ProductId: f.productID, Quantity: 1,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	var invoiceID string
	if err := f.pool.QueryRow(ctx, `SELECT xendit_invoice_id FROM orders WHERE id = $1`, response.Order.Id).Scan(&invoiceID); err != nil {
		t.Fatalf("read invoice id: %v", err)
	}
	if err := f.orders.SettlePayment(ctx, invoiceID, 0); err != nil {
		t.Fatalf("settle: %v", err)
	}
	var status string
	if err := f.pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, response.Order.Id).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "HELD" {
		t.Fatalf("status = %s with no reported amount, want HELD", status)
	}
}

// The matching case must still work, or the check would be a very effective
// way of never taking any money at all.
func TestMatchingPaymentSettlesIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()

	response, err := f.orders.CreateOrder(ctx, &hajjv1.CreateOrderRequest{
		AppAccessCode: f.accessCode(t), ProductId: f.productID, Quantity: 1,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	var invoiceID string
	if err := f.pool.QueryRow(ctx, `SELECT xendit_invoice_id FROM orders WHERE id = $1`, response.Order.Id).Scan(&invoiceID); err != nil {
		t.Fatalf("read invoice id: %v", err)
	}
	if err := f.orders.SettlePayment(ctx, invoiceID, referralPrice); err != nil {
		t.Fatalf("settle: %v", err)
	}
	var status string
	var paidAmount *int64
	if err := f.pool.QueryRow(ctx, `SELECT status, paid_amount_idr FROM orders WHERE id = $1`,
		response.Order.Id).Scan(&status, &paidAmount); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if status != "PAID" {
		t.Fatalf("status = %s for a matching payment, want PAID", status)
	}
	if paidAmount == nil || *paidAmount != referralPrice {
		t.Fatalf("recorded paid amount = %v, want %d — a settled order should carry the evidence", paidAmount, referralPrice)
	}
}

// A dropped webhook delivery used to be permanent: the jamaah had paid, the
// order sat PENDING forever, and nobody was told. This is the path that makes
// it survivable — the same settlement the webhook uses, reached without one.
func TestPollingSettlesATransactionWhoseWebhookNeverArrivedIntegration(t *testing.T) {
	f := newReferralFixture(t)
	ctx := context.Background()
	orders := repository.NewOrderRepository(db.New(f.pool))

	response, err := f.orders.CreateOrder(ctx, &hajjv1.CreateOrderRequest{
		AppAccessCode: f.accessCode(t), ProductId: f.productID, Quantity: 1,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// No webhook arrives at all. Fresh orders are left alone, so the poller
	// does not race a delivery that is probably seconds away.
	waiting, err := orders.ListAwaitingSettlement(ctx, 5, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, entry := range waiting {
		if entry.OrderID == response.Order.Id {
			t.Fatal("a just-created order was picked up before the grace period")
		}
	}

	// Age it past the grace period.
	if _, err := f.pool.Exec(ctx, `UPDATE orders SET created_at = NOW() - INTERVAL '30 minutes' WHERE id = $1`, response.Order.Id); err != nil {
		t.Fatalf("age order: %v", err)
	}
	waiting, err = orders.ListAwaitingSettlement(ctx, 5, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found string
	for _, entry := range waiting {
		if entry.OrderID == response.Order.Id {
			found = entry.InvoiceID
		}
	}
	if found == "" {
		t.Fatal("an order waiting past the grace period was not picked up")
	}

	// Settling through the poller's path reaches the same place the webhook
	// would have.
	if err := f.orders.SettleFromGateway(ctx, found); err != nil {
		t.Fatalf("settle: %v", err)
	}
	var status string
	if err := f.pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, response.Order.Id).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "PAID" {
		t.Fatalf("status = %s after polling a paid invoice, want PAID", status)
	}

	// And it drops out of the queue, so it is not polled forever.
	waiting, err = orders.ListAwaitingSettlement(ctx, 5, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, entry := range waiting {
		if entry.OrderID == response.Order.Id {
			t.Fatal("a settled order is still being polled")
		}
	}
}

// A held transaction must be resolvable from the application. Before this it
// was a dead end that needed someone with database access.
func TestResolvingAHeldTransactionIntegration(t *testing.T) {
	ctx := context.Background()
	ledger := func(f *referralFixture) *repository.LedgerRepository { return repository.NewLedgerRepository(f.pool) }

	hold := func(t *testing.T, f *referralFixture) string {
		t.Helper()
		response, err := f.orders.CreateOrder(ctx, &hajjv1.CreateOrderRequest{
			AppAccessCode: f.accessCode(t), ProductId: f.productID, Quantity: 1,
			IdempotencyKey: uuid.NewString(),
		})
		if err != nil {
			t.Fatalf("checkout: %v", err)
		}
		var invoiceID string
		if err := f.pool.QueryRow(ctx, `SELECT xendit_invoice_id FROM orders WHERE id = $1`, response.Order.Id).Scan(&invoiceID); err != nil {
			t.Fatalf("read invoice id: %v", err)
		}
		if err := f.orders.SettlePayment(ctx, invoiceID, referralPrice-50_000); err != nil {
			t.Fatalf("settle short: %v", err)
		}
		return response.Order.Id
	}

	t.Run("accepting settles it and the commission becomes payable", func(t *testing.T) {
		f := newReferralFixture(t)
		orderID := hold(t, f)

		order, err := f.orders.ResolveHeldOrder(ctx, f.orgID, "staff-1", &hajjv1.ResolveHeldOrderRequest{
			OrderId: orderID, Resolution: hajjv1.HeldOrderResolution_HELD_ORDER_RESOLUTION_ACCEPT,
			Note: "kekurangan dibayar tunai",
		})
		if err != nil {
			t.Fatalf("accept: %v", err)
		}
		if order.Status != "PAID" {
			t.Fatalf("status = %s after accepting, want PAID", order.Status)
		}
		summary, err := repository.NewAgentRepository(db.New(f.pool)).GetPayoutSummary(ctx, f.operatorID, f.referrer)
		if err != nil {
			t.Fatalf("summary: %v", err)
		}
		if summary.SettledCommissionIDR != 600_000 {
			t.Fatalf("settled commission = %d after accepting, want 600000", summary.SettledCommissionIDR)
		}
	})

	t.Run("rejecting closes it and takes the commission back", func(t *testing.T) {
		f := newReferralFixture(t)
		orderID := hold(t, f)

		order, err := f.orders.ResolveHeldOrder(ctx, f.orgID, "staff-1", &hajjv1.ResolveHeldOrderRequest{
			OrderId: orderID, Resolution: hajjv1.HeldOrderResolution_HELD_ORDER_RESOLUTION_REJECT,
			Note: "dana dikembalikan",
		})
		if err != nil {
			t.Fatalf("reject: %v", err)
		}
		if order.Status != "FAILED" {
			t.Fatalf("status = %s after rejecting, want FAILED", order.Status)
		}
		if balance, err := ledger(f).CommissionBalance(ctx, f.referrer); err != nil || balance != 0 {
			t.Fatalf("commission = %d (%v) after rejecting, want 0", balance, err)
		}
	})

	t.Run("only a held transaction can be resolved, and only once", func(t *testing.T) {
		f := newReferralFixture(t)
		orderID := hold(t, f)

		if _, err := f.orders.ResolveHeldOrder(ctx, f.orgID, "staff-1", &hajjv1.ResolveHeldOrderRequest{
			OrderId: orderID, Resolution: hajjv1.HeldOrderResolution_HELD_ORDER_RESOLUTION_ACCEPT,
		}); err != nil {
			t.Fatalf("first resolve: %v", err)
		}
		if _, err := f.orders.ResolveHeldOrder(ctx, f.orgID, "staff-1", &hajjv1.ResolveHeldOrderRequest{
			OrderId: orderID, Resolution: hajjv1.HeldOrderResolution_HELD_ORDER_RESOLUTION_REJECT,
		}); err == nil {
			t.Fatal("a resolved transaction was resolved a second time")
		}
	})
}
