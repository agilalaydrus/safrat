package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
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

// Digital products are platform-owned, so they carry no operator — and they
// are the only products that have a supplier at all. If setting a cost
// required an operator, the cost of every product that actually has one could
// never be recorded.
func TestManualCostOnAPlatformProductIntegration(t *testing.T) {
	pool, _, _ := supplierCostFixture(t)
	ctx := context.Background()

	productID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO products (id, operator_id, season_id, name, category, price_idr, base_price_idr, code)
	      VALUES ($1,NULL,NULL,'Pulsa Platform','PPOB_CREDIT',11000,10000,$2)`,
		productID, "PLAT-"+productID[:8]); err != nil {
		t.Fatalf("insert platform product: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM products WHERE id = $1`, productID)
	})

	// Empty operator: this product belongs to nobody.
	if err := NewSupplierCostRepository(pool).SetManualCost(ctx, "", productID, 9_500); err != nil {
		t.Fatalf("harga modal produk platform ditolak: %v", err)
	}

	var cost int64
	var source string
	if err := pool.QueryRow(ctx, `SELECT supplier_cost_idr, supplier_cost_source FROM products WHERE id = $1`, productID).
		Scan(&cost, &source); err != nil {
		t.Fatalf("read cost: %v", err)
	}
	if cost != 9_500 || source != "MANUAL" {
		t.Fatalf("harga modal = %d/%s, mau 9500/MANUAL", cost, source)
	}
}

// SavePlatformProduct is how the digital catalogue gets its rows, and it had
// no test at all. These are the properties that matter: what it creates
// belongs to nobody, it refuses a duplicate code, and it cannot be pointed at
// a travel's own product.
func TestSavePlatformProductIntegration(t *testing.T) {
	pool, operatorID, tenantProductID := supplierCostFixture(t)
	ctx := context.Background()
	products := NewProductRepository(db.New(pool), pool)

	base := int64(10_000)
	nominal := int64(10_000)
	code := "PLAT-" + uuid.NewString()[:8]

	created, err := products.SavePlatformProduct(ctx, "", domain.Product{
		Name: "Pulsa Platform", Code: code, Category: "PPOB_CREDIT",
		NominalIDR: &nominal, BasePriceIDR: &base, IsActive: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM products WHERE id = $1`, created.ID)
	})

	// Belongs to nobody: that is what makes it shared, and what keeps every
	// tenant write out of reach of it.
	if !created.IsPlatformOwned() {
		t.Fatalf("produk platform membawa operator %q", created.OperatorID)
	}
	if created.SeasonID != "" {
		t.Fatalf("produk platform membawa musim %q", created.SeasonID)
	}

	// A second product with the same code would make the code meaningless —
	// it exists so a person can quote one thing unambiguously.
	if _, err := products.SavePlatformProduct(ctx, "", domain.Product{
		Name: "Pulsa Kembar", Code: code, Category: "PPOB_CREDIT",
		BasePriceIDR: &base, IsActive: true,
	}); !errors.Is(err, apperror.ErrAlreadyExists) {
		t.Fatalf("kode ganda dikembalikan %v, mau ErrAlreadyExists", err)
	}

	// Editing works, and the row stays platform-owned.
	updatedBase := int64(12_000)
	updated, err := products.SavePlatformProduct(ctx, created.ID, domain.Product{
		Name: "Pulsa Platform 12K", Code: code, Category: "PPOB_CREDIT",
		NominalIDR: &nominal, BasePriceIDR: &updatedBase, IsActive: true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.BasePriceIDR == nil || *updated.BasePriceIDR != updatedBase {
		t.Fatalf("harga dasar tidak tersimpan: %v", updated.BasePriceIDR)
	}
	if !updated.IsPlatformOwned() {
		t.Fatal("penyuntingan memberi produk platform seorang pemilik")
	}

	// The guard that matters most: handed a travel's own product id, this must
	// refuse rather than quietly rewriting a tenant's catalogue row into a
	// platform one.
	if _, err := products.SavePlatformProduct(ctx, tenantProductID, domain.Product{
		Name: "Bajakan", Code: "BAJAK-" + uuid.NewString()[:6], Category: "PPOB_CREDIT",
		BasePriceIDR: &base, IsActive: true,
	}); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("produk milik travel dapat disunting dari jalur platform: %v", err)
	}

	// The catalogue lists what the platform supplies and nothing else. A
	// tenant's product appearing here would be one travel's catalogue shown to
	// the admin as if it were shared by all of them.
	catalogue, err := products.PlatformCatalogue(ctx)
	if err != nil {
		t.Fatalf("catalogue: %v", err)
	}
	var found bool
	for _, item := range catalogue {
		if item.ID == tenantProductID {
			t.Fatal("katalog platform memuat produk milik travel")
		}
		if !item.IsPlatformOwned() {
			t.Fatalf("katalog memuat produk bertuan: %s", item.ID)
		}
		if item.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("produk yang baru dibuat tidak muncul di katalog")
	}

	// And it really is untouched.
	var owner string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(operator_id::text,'') FROM products WHERE id = $1`, tenantProductID).Scan(&owner); err != nil {
		t.Fatalf("read tenant product: %v", err)
	}
	if owner != operatorID {
		t.Fatalf("produk travel berpindah tangan: pemilik = %q", owner)
	}
}
