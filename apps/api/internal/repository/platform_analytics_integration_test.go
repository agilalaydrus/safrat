package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MRR, and the four kinds of tenant that must not be counted in it.
//
// The number is only useful if it means "what we are actually being paid this
// month". A tenant on trial, one who cancelled, one we suspended, and one whose
// paid time has run out are all still rows in the table — and counting any of
// them makes the business look healthier than it is at exactly the moment that
// matters.
func TestPlatformMRRCountsOnlyTenantsWhoArePayingIntegration(t *testing.T) {
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

	// The window's figures are computed across every tenant in the database, so
	// the test measures the difference this fixture makes rather than absolute
	// totals another test's rows would disturb.
	repo := NewPlatformRepository(pool)
	before, err := repo.Analytics(ctx, 30)
	if err != nil {
		t.Fatalf("analytics sebelum: %v", err)
	}

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	suffix := uuid.NewString()[:8]
	newTenant := func(tag, plan, status string, extra string) string {
		id := uuid.NewString()
		exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
			VALUES ($1,$2,$3,'ID',$4,$5,$6)`, id, tag+"-"+suffix, "Uji "+tag,
			tag+"-"+suffix+"@example.test", tag+"-"+suffix, plan)
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, id) })
		exec(`INSERT INTO subscriptions (operator_id,plan,status,access_until,grace_period_days)
			VALUES ($1,$2::plan,$3::subscription_status, NOW() + INTERVAL '20 days', 0)`,
			id, plan, status)
		if extra != "" {
			exec(`UPDATE subscriptions SET `+extra+` WHERE operator_id = $1`, id)
		}
		return id
	}

	var growthPrice, proPrice int64
	if err := pool.QueryRow(ctx, `SELECT monthly_idr FROM plan_prices WHERE plan::text = 'GROWTH'`).Scan(&growthPrice); err != nil {
		t.Fatalf("harga GROWTH: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT monthly_idr FROM plan_prices WHERE plan::text = 'PRO'`).Scan(&proPrice); err != nil {
		t.Fatalf("harga PRO: %v", err)
	}

	// One paying tenant, and four that must not count.
	newTenant("bayar", "GROWTH", "ACTIVE", "")
	newTenant("trial", "PRO", "TRIALING", "")
	newTenant("batal", "PRO", "ACTIVE", "cancelled_at = NOW()")
	newTenant("tangguh", "PRO", "ACTIVE", "suspended_at = NOW()")
	newTenant("habis", "PRO", "ACTIVE", "access_until = NOW() - INTERVAL '5 days'")

	after, err := repo.Analytics(ctx, 30)
	if err != nil {
		t.Fatalf("analytics sesudah: %v", err)
	}

	if delta := after.MRRIDR - before.MRRIDR; delta != growthPrice {
		t.Fatalf("MRR bertambah %d, mau %d (hanya satu tenant yang benar-benar membayar)", delta, growthPrice)
	}
	if delta := after.PayingTenants - before.PayingTenants; delta != 1 {
		t.Fatalf("tenant membayar bertambah %d, mau 1", delta)
	}
	if delta := after.TrialingTenants - before.TrialingTenants; delta != 1 {
		t.Fatalf("tenant trial bertambah %d, mau 1", delta)
	}
	if delta := after.SuspendedTenants - before.SuspendedTenants; delta != 1 {
		t.Fatalf("tenant ditangguhkan bertambah %d, mau 1", delta)
	}
	if delta := after.LapsedTenants - before.LapsedTenants; delta != 1 {
		t.Fatalf("tenant habis masa bertambah %d, mau 1", delta)
	}
	// The cancelled tenant is churn, valued at what they had been paying.
	if delta := after.ChurnedMRRIDR - before.ChurnedMRRIDR; delta != proPrice {
		t.Fatalf("MRR churn bertambah %d, mau %d", delta, proPrice)
	}

	// New MRR only counts the one that is paying — a trial that started in the
	// window is not new revenue until it pays.
	if delta := after.NewMRRIDR - before.NewMRRIDR; delta != growthPrice {
		t.Fatalf("MRR baru bertambah %d, mau %d", delta, growthPrice)
	}

	// A plan upgrade is expansion, and it is read from the change ledger.
	upgraded := newTenant("naik", "PRO", "ACTIVE", "")
	exec(`INSERT INTO subscription_adjustments
		(operator_id,kind,from_plan,to_plan,amount_idr,effective_at,access_until_snapshot,
		 remaining_seconds,period_seconds,reason,requested_by,idempotency_key,request_fingerprint)
		VALUES ($1,'PRORATION_DEBIT','GROWTH','PRO',1000000,NOW(),NOW()+INTERVAL '20 days',1728000,2592000,'uji ekspansi','uji',$2,'uji')`,
		upgraded, "exp-"+uuid.NewString())
	expanded, err := repo.Analytics(ctx, 30)
	if err != nil {
		t.Fatalf("analytics ekspansi: %v", err)
	}
	if delta := expanded.ExpansionMRRIDR - after.ExpansionMRRIDR; delta != proPrice-growthPrice {
		t.Fatalf("ekspansi bertambah %d, mau %d", delta, proPrice-growthPrice)
	}
	if expanded.ContractionMRRIDR != after.ContractionMRRIDR {
		t.Fatal("kenaikan paket ikut terhitung sebagai penurunan")
	}
}
