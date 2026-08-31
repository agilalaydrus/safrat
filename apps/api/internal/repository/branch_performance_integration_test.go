package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBranchPerformanceIsAggregatedAndScopedIntegration(t *testing.T) {
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
	bandungID, medanID := uuid.NewString(), uuid.NewString()
	bandungPilgrimID, medanPilgrimID := uuid.NewString(), uuid.NewString()
	bandungHead := "performance-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug)
	      VALUES ($1,$2,'Performance Test','ID',$3,$4)`, operatorID,
		"performance-"+uuid.NewString(), operatorID[:8]+"@example.test", "performance-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, operatorID) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim Performance','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	exec(`INSERT INTO branches (id, operator_id, name, city, target_pilgrims, target_revenue_idr)
	      VALUES ($1,$3,'Bandung','Bandung',2,200000),($2,$3,'Medan','Medan',4,600000)`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)
	exec(`INSERT INTO agents (id, operator_id, name, branch_id) VALUES ($1,$2,'Agen Bandung',$3),($4,$2,'Agen Medan',$5)`,
		uuid.NewString(), operatorID, bandungID, uuid.NewString(), medanID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, full_name, passport_number, nationality, date_of_birth, gender, payment_status, documents_passport, documents_photo, documents_vaccine)
	      VALUES ($1,$3,$4,$5,'Jamaah Bandung','PERF-BDG','ID','1990-01-01','MALE','PAID',true,true,true),
	             ($2,$3,$4,$6,'Jamaah Medan','PERF-MDN','ID','1991-01-01','FEMALE','DP',true,false,false)`,
		bandungPilgrimID, medanPilgrimID, seasonID, operatorID, bandungID, medanID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, type, price_idr)
	      VALUES ($1,$2,$3,'Produk Performance','UMRAH',100000)`, productID, operatorID, seasonID)

	orders := NewOrderRepository(db.New(pool), pool)
	createOrder := func(requestCtx context.Context, pilgrimID, key string) string {
		t.Helper()
		order, created, err := orders.Create(requestCtx, CreateOrderParams{
			OperatorID: operatorID, SeasonID: seasonID, PilgrimID: pilgrimID, ProductID: productID,
			BuyerKind: "PILGRIM", Quantity: 1, UnitPriceIDR: 100000, TotalPriceIDR: 100000,
			OperatorAmountIDR: 100000, CheckoutChannel: "MANUAL", IdempotencyKey: key,
		})
		if err != nil || !created {
			t.Fatalf("create order: %#v created=%v err=%v", order, created, err)
		}
		if _, err := orders.MarkPaidManually(ctx, operatorID, order.ID); err != nil {
			t.Fatalf("mark paid: %v", err)
		}
		return order.ID
	}
	createOrder(ContextWithStaffActor(ctx, bandungHead), bandungPilgrimID, "performance-bdg-"+uuid.NewString())
	createOrder(ctx, medanPilgrimID, "performance-mdn-"+uuid.NewString())

	branches := NewBranchRepository(db.New(pool))
	bandungReport, err := branches.GetPerformance(ContextWithStaffActor(ctx, bandungHead), operatorID, seasonID)
	if err != nil || len(bandungReport.Branches) != 1 {
		t.Fatalf("laporan Bandung bocor: %#v (%v)", bandungReport, err)
	}
	bandung := bandungReport.Branches[0]
	if bandung.BranchID != bandungID || bandung.RevenueIDR != 100000 || bandung.PilgrimCount != 1 || bandung.AgentCount != 1 || bandung.CollectionPct != 100 || bandung.DocumentsReadyPct != 100 || len(bandung.Trend) != 12 {
		t.Fatalf("agregat Bandung salah: %#v", bandung)
	}
	if bandungReport.NetworkRevenueIDR != 100000 || bandungReport.NetworkPilgrimCount != 1 || bandungReport.BelowTargetCount != 1 {
		t.Fatalf("jaringan Bandung tidak dibatasi: %#v", bandungReport)
	}
	headOffice, err := branches.GetPerformance(ContextWithStaffActor(ctx, "performance-hq-"+uuid.NewString()), operatorID, seasonID)
	if err != nil || len(headOffice.Branches) != 2 || headOffice.NetworkRevenueIDR != 200000 || headOffice.NetworkPilgrimCount != 2 {
		t.Fatalf("laporan pusat salah: %#v (%v)", headOffice, err)
	}
}
