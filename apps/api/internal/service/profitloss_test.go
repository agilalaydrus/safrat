package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Net profit must be revenue minus platform fee minus agent commission minus
// known cost — not revenue minus cost alone, which would count the
// platform's and the agent's share as if it belonged to the operator. And an
// order whose product has no known supplier_cost_idr must be excluded from
// the cost total, not silently treated as zero cost (which would inflate
// profit for exactly the products nobody has priced yet).
func TestProfitLossNetProfitAndMissingCostIntegration(t *testing.T) {
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

	operatorID, orgID := uuid.NewString(), "pnl-"+uuid.NewString()
	seasonID := uuid.NewString()
	pilgrimID := uuid.NewString()
	productWithCostID, productNoCostID := uuid.NewString(), uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'PnL Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "pnl-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`,
		seasonID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,'Jamaah Uji','PNL12345','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimID, seasonID, operatorID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, price_idr, supplier_cost_idr, supplier_cost_source) VALUES ($1,$2,$3,'Produk Berbiaya',1000000,600000,'MANUAL')`,
		productWithCostID, operatorID, seasonID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, price_idr) VALUES ($1,$2,$3,'Produk Tanpa Biaya',1000000)`,
		productNoCostID, operatorID, seasonID)

	// Known-cost order: revenue 1,000,000, platform 100,000, agent 0, cost 600,000.
	// Net profit must be 1,000,000 - 100,000 - 0 - 600,000 = 300,000.
	exec(`INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity, unit_price_idr, total_price_idr, platform_amount_idr, operator_amount_idr, agent_commission_idr, status, paid_at)
	      VALUES ($1,$2,$3,$4,1,1000000,1000000,100000,900000,0,'PAID',NOW())`,
		operatorID, seasonID, pilgrimID, productWithCostID)
	// Unknown-cost order: same revenue, must NOT contribute to cost_idr, and
	// must be counted in orders_missing_cost / revenue_missing_cost_idr.
	exec(`INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity, unit_price_idr, total_price_idr, platform_amount_idr, operator_amount_idr, agent_commission_idr, status, paid_at)
	      VALUES ($1,$2,$3,$4,1,1000000,1000000,100000,900000,0,'PAID',NOW())`,
		operatorID, seasonID, pilgrimID, productNoCostID)
	// A refunded order in the same window must not count as revenue at all.
	exec(`INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity, unit_price_idr, total_price_idr, platform_amount_idr, operator_amount_idr, agent_commission_idr, status, paid_at)
	      VALUES ($1,$2,$3,$4,1,5000000,5000000,500000,4500000,0,'REFUNDED',NOW())`,
		operatorID, seasonID, pilgrimID, productWithCostID)

	t.Cleanup(func() {
		exec(`DELETE FROM orders WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM products WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM pilgrims WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM seasons WHERE id = $1`, seasonID)
		exec(`DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	profitLossService := NewProfitLossService(repository.NewOperatorRepository(queries), repository.NewProfitLossRepository(pool))

	report, err := profitLossService.GetProfitLossReport(ctx, orgID, &hajjv1.GetProfitLossReportRequest{Months: 1})
	if err != nil {
		t.Fatalf("GetProfitLossReport: %v", err)
	}
	if len(report.Periods) != 1 {
		t.Fatalf("jumlah periode = %d, mau 1: %+v", len(report.Periods), report.Periods)
	}
	period := report.Periods[0]

	if period.RevenueIdr != 2000000 {
		t.Fatalf("revenue = %d, mau 2000000 (refund tidak boleh ikut terhitung)", period.RevenueIdr)
	}
	if period.CostIdr != 600000 {
		t.Fatalf("cost = %d, mau 600000 (hanya produk berbiaya diketahui)", period.CostIdr)
	}
	if period.OrdersMissingCost != 1 || period.RevenueMissingCostIdr != 1000000 {
		t.Fatalf("orders_missing_cost=%d revenue_missing_cost=%d, mau 1 dan 1000000", period.OrdersMissingCost, period.RevenueMissingCostIdr)
	}
	// Net = revenue(2,000,000) - platform(200,000) - agent(0) - cost(600,000) = 1,200,000.
	if period.NetProfitIdr != 1200000 {
		t.Fatalf("net_profit = %d, mau 1200000", period.NetProfitIdr)
	}

	// The export must see exactly the two PAID orders — not the REFUNDED one
	// — proving StreamExport's own filter matches the aggregate query's.
	rowsSeen := 0
	if err := repository.NewProfitLossRepository(pool).StreamExport(ctx, operatorID, time.Now().Add(-24*time.Hour), func(row domain.ExportRow) error {
		rowsSeen++
		return nil
	}); err != nil {
		t.Fatalf("StreamExport: %v", err)
	}
	if rowsSeen != 2 {
		t.Fatalf("baris ekspor terlihat = %d, mau 2 (dua pesanan PAID; yang REFUNDED tidak ikut)", rowsSeen)
	}
}
