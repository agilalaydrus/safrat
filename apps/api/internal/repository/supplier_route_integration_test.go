package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A product with no supplier route is the one that matters: an order for it
// comes back "Produk Belum di Atur Routing", and until this listing showed it,
// the only way to find one was to read the database by hand.
//
// The listing used to start FROM product_routes, so an unrouted product could
// not appear in it at all. This drives the case both ways — the unrouted
// product is present and first, the routed one carries its supplier, and a
// travel package is left out entirely.
func TestUnroutedDigitalProductsAreListedFirstIntegration(t *testing.T) {
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

	suffix := uuid.NewString()[:8]
	routedID, unroutedID, packageID, supplierID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	operatorID, seasonID, pilgrimID, orderID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	// Digital products are platform-owned (migration 114), so only the travel
	// package needs an operator and a season to hang from.
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
	      VALUES ($1,$2,'Rute Uji','ID',$3,$4)`,
		operatorID, "route-"+suffix, "route-"+suffix+"@example.test", "route-"+suffix)
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
	      VALUES ($1,$2,'Musim Rute','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID)
	exec(`INSERT INTO suppliers (id,code,name,base_url)
	      VALUES ($1,$2,'Supplier Rute','https://supplier.test')`, supplierID, "SUP-"+suffix)
	exec(`INSERT INTO products (id,name,code,category,price_idr,base_price_idr,is_active)
	      VALUES ($1,'Zzz Pulsa Terhubung',$2,'PPOB_CREDIT',10000,10000,true),
	             ($3,'Aaa Pulsa Belum Dirutekan',$4,'PPOB_CREDIT',10000,10000,true)`,
		routedID, "RT-"+suffix, unroutedID, "UR-"+suffix)
	exec(`INSERT INTO products (id,operator_id,season_id,name,code,category,price_idr,is_active)
	      VALUES ($1,$2,$3,'Paket Umroh Uji',$4,'TRAVEL_PACKAGE',25000000,true)`,
		packageID, operatorID, seasonID, "PK-"+suffix)
	exec(`INSERT INTO product_routes (product_id,supplier_id,supplier_sku,is_active)
	      VALUES ($1,$2,'SKU-1',true)`, routedID, supplierID)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,full_name,passport_number,nationality,date_of_birth,gender)
	      VALUES ($1,$2,$3,'Jamaah Log',$4,'ID','1990-01-01','MALE')`, pilgrimID, seasonID, operatorID, "LOG-"+suffix)
	exec(`INSERT INTO orders (id,operator_id,season_id,pilgrim_id,product_id,unit_price_idr,total_price_idr,status)
	      VALUES ($1,$2,$3,$4,$5,25000000,25000000,'PAID')`, orderID, operatorID, seasonID, pilgrimID, packageID)
	exec(`INSERT INTO supplier_request_logs (supplier_id,order_id,direction,endpoint,request_body,response_body,outcome)
	      VALUES ($1,$2,'REQUEST','/order','{}','{"status":"pending"}','PENDING'),
	             ($1,NULL,'CALLBACK','/callback','{}','{"status":"other"}','UNMATCHED')`, supplierID, orderID)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM supplier_request_logs WHERE supplier_id = $1`, supplierID)
		_, _ = pool.Exec(bg, `DELETE FROM product_routes WHERE product_id IN ($1,$2)`, routedID, unroutedID)
		_, _ = pool.Exec(bg, `DELETE FROM products WHERE id IN ($1,$2,$3)`, routedID, unroutedID, packageID)
		_, _ = pool.Exec(bg, `DELETE FROM suppliers WHERE id = $1`, supplierID)
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, operatorID)
	})

	routes, err := NewSupplierRepository(pool).ListRoutes(ctx)
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}

	var routed, unrouted *ProductRoute
	firstUnroutedAt, firstRoutedAt := -1, -1
	for index, route := range routes {
		switch route.ProductID {
		case routedID:
			routed = route
		case unroutedID:
			unrouted = route
		case packageID:
			t.Fatalf("paket umroh muncul di daftar routing pada posisi %d", index)
		}
		if route.ID == "" && firstUnroutedAt < 0 {
			firstUnroutedAt = index
		}
		if route.ID != "" && firstRoutedAt < 0 {
			firstRoutedAt = index
		}
	}

	if unrouted == nil {
		t.Fatal("produk tanpa routing tidak muncul sama sekali — inilah yang dulu tersembunyi")
	}
	if unrouted.ID != "" || unrouted.SupplierID != "" || unrouted.SupplierName != "" {
		t.Fatalf("baris tanpa routing membawa supplier: %#v", unrouted)
	}
	if routed == nil || routed.SupplierID != supplierID || routed.SupplierSKU != "SKU-1" || !routed.IsActive {
		t.Fatalf("baris ber-routing tidak lengkap: %#v", routed)
	}
	// Named so the routed one sorts first alphabetically: if ordering were by
	// name alone this would fail, which is the point.
	if firstRoutedAt >= 0 && firstUnroutedAt > firstRoutedAt {
		t.Fatalf("yang belum dirutekan tidak di atas: unrouted=%d routed=%d", firstUnroutedAt, firstRoutedAt)
	}

	logs, err := NewSupplierRepository(pool).ListLogs(ctx, false, orderID, 20)
	if err != nil || len(logs) != 1 || logs[0].OrderID != orderID {
		t.Fatalf("filter log transaksi tidak tepat: %#v (%v)", logs, err)
	}
	_ = db.New(pool)
}
