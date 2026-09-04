package service

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Room tiers had a hole from the day they shipped: the quota trigger and the
// tier catalogue (T3.4) existed, but nothing in ordinary checkout ever priced
// a tier or wrote room_tier onto the order — so the trigger, which only fires
// when room_tier is set, silently never ran, and a DOUBLE tier was billed at
// the plain package price. This is the fix, and the test that would have
// caught its absence.
func TestCreateManualOrderPricesAndEnforcesRoomTierIntegration(t *testing.T) {
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
			t.Fatalf("fixture: %v", err)
		}
	}
	operatorID, orgID := uuid.NewString(), "tier-order-"+uuid.NewString()
	suffix := uuid.NewString()[:8]
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Tier Pesanan','ID',$3,$4)`, operatorID, orgID, "tord-"+suffix+"@example.test", "tord-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	seasonID, productID := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
		VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, seasonID, operatorID)
	exec(`INSERT INTO products (id,operator_id,season_id,name,price_idr,base_price_idr)
		VALUES ($1,$2,$3,'Paket Uji Tier',30000000,30000000)`, productID, operatorID, seasonID)
	exec(`INSERT INTO product_markups (product_id,operator_id,operator_markup_idr,agent_markup_idr)
		VALUES ($1,$2,0,0)`, productID, operatorID)
	quota := int32(1)
	queries := db.New(pool)
	products := repository.NewProductRepository(queries, pool)
	if _, _, err := products.SetRoomTiers(ctx, operatorID, productID, []repository.RoomTier{
		{Tier: "DOUBLE", PriceDeltaIDR: 5_000_000, SeatQuota: &quota, IsActive: true},
	}); err != nil {
		t.Fatalf("set room tiers: %v", err)
	}

	newPilgrim := func(name string) string {
		id := uuid.NewString()
		exec(`INSERT INTO pilgrims (id,season_id,operator_id,full_name,passport_number,nationality,date_of_birth,gender)
			VALUES ($1,$2,$3,$4,$5,'ID','1990-01-01'::timestamptz,'MALE')`,
			id, seasonID, operatorID, name, "P-"+uuid.NewString()[:8])
		return id
	}
	pilgrimA, pilgrimB := newPilgrim("Jamaah Pertama"), newPilgrim("Jamaah Kedua")

	orders := NewOrderService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		products, repository.NewOrderRepository(queries, pool),
		repository.NewAuditRepository(queries), repository.NewLedgerRepository(pool),
		repository.NewRefundRepository(pool), repository.NewAgentRepository(queries), repository.NewSeasonRepository(queries),
		pool, payment.NewClient(""), "http://localhost:3000")

	// The price must include the tier delta: Rp30.000.000 + Rp5.000.000.
	first, err := orders.CreateManualOrder(ctx, orgID, &hajjv1.CreateManualOrderRequest{
		PilgrimId: pilgrimA, ProductId: productID, Quantity: 1,
		PaymentMethod:  hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_CASH,
		RoomTier:       "DOUBLE",
		IdempotencyKey: "tier-order-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("pesanan pertama: %v", err)
	}
	if first.Order.TotalPriceIdr != 35_000_000 {
		t.Fatalf("total = %d, mau 35.000.000 — selisih tier tidak ikut dihitung", first.Order.TotalPriceIdr)
	}
	if first.Order.RoomTier != "DOUBLE" {
		t.Fatalf("room_tier tersimpan %q, mau DOUBLE — pemicu kuota tidak akan pernah menyala", first.Order.RoomTier)
	}

	// The quota is 1 and it is now taken. A second purchase of the same tier
	// must be refused by the database trigger, not silently allowed.
	_, err = orders.CreateManualOrder(ctx, orgID, &hajjv1.CreateManualOrderRequest{
		PilgrimId: pilgrimB, ProductId: productID, Quantity: 1,
		PaymentMethod:  hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_CASH,
		RoomTier:       "DOUBLE",
		IdempotencyKey: "tier-order-" + uuid.NewString(),
	})
	if err == nil {
		t.Fatal("kursi DOUBLE terjual dua kali untuk kuota 1 — celah T3.4 belum tertutup")
	}

	// A tier the product does not offer is refused before anything is written.
	_, err = orders.CreateManualOrder(ctx, orgID, &hajjv1.CreateManualOrderRequest{
		PilgrimId: pilgrimB, ProductId: productID, Quantity: 1,
		PaymentMethod:  hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_CASH,
		RoomTier:       "TRIPLE",
		IdempotencyKey: "tier-order-" + uuid.NewString(),
	})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("tier yang tidak ditawarkan = %v, mau failed_precondition", connect.CodeOf(err))
	}

	// No tier at all still works, and prices at the plain package total —
	// tiers must stay optional for products that have none.
	plain, err := orders.CreateManualOrder(ctx, orgID, &hajjv1.CreateManualOrderRequest{
		PilgrimId: pilgrimB, ProductId: productID, Quantity: 1,
		PaymentMethod:  hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_CASH,
		IdempotencyKey: "tier-order-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("pesanan tanpa tier: %v", err)
	}
	if plain.Order.TotalPriceIdr != 30_000_000 || plain.Order.RoomTier != "" {
		t.Fatalf("pesanan tanpa tier salah: total=%d tier=%q", plain.Order.TotalPriceIdr, plain.Order.RoomTier)
	}
}
