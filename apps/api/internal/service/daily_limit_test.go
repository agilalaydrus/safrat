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

	"connectrpc.com/connect"
	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Each purchase is Rp9 juta against a Rp20 juta cap, so two fit and the third
// cannot. Deliberately not a divisor of the limit: a purchase that divides it
// exactly would let an off-by-one in either direction still look correct.
const limitTestPrice = int64(9_000_000)

type limitFixture struct {
	pool       *pgxpool.Pool
	orders     *OrderService
	operatorID string
	orgID      string
	productID  string
	pilgrimID  string
	accessCode string
}

func newLimitFixture(t *testing.T) *limitFixture {
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

	operatorID, orgID := uuid.NewString(), "limit-"+uuid.NewString()
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Batas Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "lim-"+operatorID[:8])

	supplierID := uuid.NewString()
	seasonID, productID, pilgrimID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO suppliers (id, name, code, base_url, status) VALUES ($1,'Supplier Uji',$2,'https://stub.invalid','ACTIVE')`,
		supplierID, "SUP-"+operatorID[:8])

	// Cleaned up in reverse dependency order. The supplier is global rather
	// than tenant-owned, so deleting the operator does not take it — leaving it
	// behind would accumulate a row per test run, which is the leak this suite
	// has been bitten by before.
	t.Cleanup(func() {
		bg := context.Background()
		cleanup, err := pool.Begin(bg)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Rollback(bg) }()
		if _, err := cleanup.Exec(bg, `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
			return
		}
		if _, err := cleanup.Exec(bg, `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		// Order matters and is easy to get backwards. The product's route holds
		// the supplier with ON DELETE RESTRICT, so the product has to go first
		// or the supplier delete fails — and one failed statement rolls the
		// whole cleanup back, leaving everything behind rather than some of it.
		//
		// operators (cascades orders and pilgrims)
		//   -> products (cascades routes and markups)
		//     -> suppliers
		if _, err := cleanup.Exec(bg, `DELETE FROM products WHERE id = $1`, productID); err != nil {
			return
		}
		if _, err := cleanup.Exec(bg, `DELETE FROM suppliers WHERE id = $1`, supplierID); err != nil {
			return
		}
		// daily_digital_spend cannot be reached by cascade: buyer_id is
		// polymorphic (a pilgrim or an agent), so it carries no foreign key.
		if _, err := cleanup.Exec(bg, `DELETE FROM daily_digital_spend WHERE buyer_id = $1`, pilgrimID); err != nil {
			return
		}
		_ = cleanup.Commit(bg)
	})

	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID)
	// Platform-owned, like every digital product now is.
	exec(`INSERT INTO products (id, operator_id, season_id, name, category, price_idr, base_price_idr, nominal_idr, code)
	      VALUES ($1,NULL,NULL,'Pulsa Uji','PPOB_CREDIT',$2,$3,$4,$5)`,
		productID, limitTestPrice, limitTestPrice-1_000_000, limitTestPrice, "LIM-"+productID[:8])
	exec(`INSERT INTO product_markups (product_id, operator_id, operator_markup_idr, agent_markup_idr)
	      VALUES ($1,$2,1000000,0)`, productID, operatorID)
	// Routing, or the checkout gate refuses before the limit is ever reached
	// and this suite would be testing the wrong refusal.
	exec(`INSERT INTO product_routes (product_id, supplier_id, supplier_sku, is_active) VALUES ($1,$2,'SKU-UJI',true)`,
		productID, supplierID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender, phone)
	      VALUES ($1,$2,$3,'Jamaah Batas','P-LIM','ID','1990-01-01'::timestamptz,'MALE','081200000000')`,
		pilgrimID, seasonID, operatorID)

	invoices := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			id := r.URL.Path[1:]
			_, _ = w.Write([]byte(`{"id":"` + id + `","status":"PAID","amount":` +
				strconv.FormatInt(limitTestPrice, 10) + `,"paid_amount":` + strconv.FormatInt(limitTestPrice, 10) + `}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"inv-` + uuid.NewString() + `","invoice_url":"https://stub.invalid/pay"}`))
	}))
	t.Cleanup(invoices.Close)

	queries := db.New(pool)
	orders := NewOrderService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		repository.NewProductRepository(queries, pool), repository.NewOrderRepository(queries, pool),
		repository.NewAuditRepository(queries), repository.NewLedgerRepository(pool),
		repository.NewRefundRepository(pool), repository.NewAgentRepository(queries), repository.NewSeasonRepository(queries),
		pool, payment.NewClientWithEndpoint("test-key", invoices.URL), "http://localhost:3000")

	var code string
	if err := pool.QueryRow(ctx, `SELECT app_access_code FROM pilgrims WHERE id = $1`, pilgrimID).Scan(&code); err != nil {
		t.Fatalf("access code: %v", err)
	}

	return &limitFixture{pool: pool, orders: orders, operatorID: operatorID, orgID: orgID,
		productID: productID, pilgrimID: pilgrimID, accessCode: code}
}

func (f *limitFixture) buy(t *testing.T) error {
	t.Helper()
	_, err := f.orders.CreateOrder(context.Background(), &hajjv1.CreateOrderRequest{
		AppAccessCode: f.accessCode, ProductId: f.productID, Quantity: 1,
		IdempotencyKey: uuid.NewString(),
	})
	return err
}

func (f *limitFixture) spentToday(t *testing.T) int64 {
	t.Helper()
	var total int64
	err := f.pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(total_idr),0) FROM daily_digital_spend WHERE buyer_id = $1`, f.pilgrimID).Scan(&total)
	if err != nil {
		t.Fatalf("read spend: %v", err)
	}
	return total
}

func TestDailyLimitRefusesTheThirdPurchaseIntegration(t *testing.T) {
	f := newLimitFixture(t)

	if err := f.buy(t); err != nil {
		t.Fatalf("pembelian pertama ditolak: %v", err)
	}
	if err := f.buy(t); err != nil {
		t.Fatalf("pembelian kedua ditolak: %v", err)
	}

	err := f.buy(t)
	if err == nil {
		t.Fatal("pembelian ketiga menembus batas Rp20 juta")
	}
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("kode %v, mau ResourceExhausted — permintaannya tidak salah, cuma kehabisan kuota", connect.CodeOf(err))
	}
	// The refusal has to carry the numbers, or a jamaah has to phone customer
	// service to ask something the system already knows.
	for _, want := range []string{"20.000.000", "18.000.000", "2.000.000"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("pesan %q tidak menyebut %s", err.Error(), want)
		}
	}

	if spent := f.spentToday(t); spent != 2*limitTestPrice {
		t.Fatalf("terpakai %d, mau %d — pembelian yang ditolak tidak boleh terhitung", spent, 2*limitTestPrice)
	}
}

// The reason the total lives in a constrained row rather than being read and
// then checked: two requests arriving together both read the old total, both
// find room, and both write.
func TestDailyLimitHoldsUnderConcurrencyIntegration(t *testing.T) {
	f := newLimitFixture(t)

	const attempts = 6
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	start := make(chan struct{})

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = f.buy(t)
		}(i)
	}
	close(start)
	wg.Wait()

	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
		}
	}

	if succeeded != 2 {
		t.Fatalf("%d pembelian lolos, mau tepat 2 — Rp20 juta hanya memuat dua kali Rp9 juta", succeeded)
	}
	if spent := f.spentToday(t); spent > 20_000_000 {
		t.Fatalf("terpakai %d melebihi batas: konkurensi menembus batas", spent)
	}
}

// Pending and paid orders both count (owner's rule), but a refunded one stops
// holding value and its headroom must come back — otherwise a refunded
// mistake costs the jamaah the rest of their day.
func TestRefundReturnsDailyHeadroomIntegration(t *testing.T) {
	f := newLimitFixture(t)
	ctx := context.Background()

	if err := f.buy(t); err != nil {
		t.Fatalf("pembelian ditolak: %v", err)
	}
	if err := f.buy(t); err != nil {
		t.Fatalf("pembelian kedua ditolak: %v", err)
	}
	if err := f.buy(t); err == nil {
		t.Fatal("pembelian ketiga seharusnya ditolak")
	}

	var orderID string
	if err := f.pool.QueryRow(ctx,
		`SELECT id::text FROM orders WHERE pilgrim_id = $1 ORDER BY created_at LIMIT 1`, f.pilgrimID).Scan(&orderID); err != nil {
		t.Fatalf("baca order: %v", err)
	}

	// Settle it first: only a paid order can be refunded.
	var invoiceID string
	if err := f.pool.QueryRow(ctx, `SELECT xendit_invoice_id FROM orders WHERE id = $1`, orderID).Scan(&invoiceID); err != nil {
		t.Fatalf("baca invoice: %v", err)
	}
	if err := f.orders.SettleFromGateway(ctx, invoiceID); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if _, err := f.orders.RefundOrder(ctx, f.orgID, "uji-user", &hajjv1.RefundOrderRequest{
		OrderId: orderID, Reason: "uji pelepasan kuota", IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatalf("refund: %v", err)
	}

	if spent := f.spentToday(t); spent != limitTestPrice {
		t.Fatalf("terpakai %d setelah refund, mau %d", spent, limitTestPrice)
	}
	if err := f.buy(t); err != nil {
		t.Fatalf("pembelian setelah refund ditolak, kuota tidak kembali: %v", err)
	}
}

// A retried checkout must not spend twice. The idempotency key returns the
// same order, and the limit has to agree.
func TestReplayedCheckoutSpendsOnceIntegration(t *testing.T) {
	f := newLimitFixture(t)
	key := uuid.NewString()

	buy := func() error {
		_, err := f.orders.CreateOrder(context.Background(), &hajjv1.CreateOrderRequest{
			AppAccessCode: f.accessCode, ProductId: f.productID, Quantity: 1,
			IdempotencyKey: key,
		})
		return err
	}

	if err := buy(); err != nil {
		t.Fatalf("pembelian ditolak: %v", err)
	}
	if err := buy(); err != nil {
		t.Fatalf("pengulangan ditolak: %v", err)
	}

	if spent := f.spentToday(t); spent != limitTestPrice {
		t.Fatalf("terpakai %d, mau %d — pengulangan terhitung dua kali", spent, limitTestPrice)
	}
}
