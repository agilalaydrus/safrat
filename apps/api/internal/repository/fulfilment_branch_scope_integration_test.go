package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFulfilmentRepositoryEnforcesShipmentBranchScopeBothWaysIntegration(t *testing.T) {
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

	op, season, product := uuid.NewString(), uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	bandungOrder, medanOrder := uuid.NewString(), uuid.NewString()
	head := "shipment-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Shipment scope','ID',$3,$4,'GROWTH')`, op, "shipment-scope-"+uuid.NewString(), op[:8]+"@example.test", "shipment-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO products (id,operator_id,season_id,name,type,category,price_idr) VALUES ($1,$2,$3,'Perlengkapan','','EQUIPMENT',100000)`, product, op, season)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,'Jamaah Bandung','SHP-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,'Jamaah Medan','SHP-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan)
	exec(`INSERT INTO orders (id,operator_id,season_id,pilgrim_id,product_id,branch_id,quantity,unit_price_idr,total_price_idr,status,paid_at,idempotency_key) VALUES ($1,$3,$4,$5,$7,$8,1,100000,100000,'PAID',NOW(),$9),($2,$3,$4,$6,$7,$10,1,100000,100000,'PAID',NOW(),$11)`, bandungOrder, medanOrder, op, season, bandungPilgrim, medanPilgrim, product, bandung, "shipment-bdg-"+uuid.NewString(), medan, "shipment-mdn-"+uuid.NewString())
	exec(`INSERT INTO order_fulfilments (order_id,operator_id,kind) VALUES ($1,$3,'SHIPMENT'),($2,$3,'SHIPMENT')`, bandungOrder, medanOrder, op)

	repo := NewFulfilmentRepository(pool)
	bandungCtx := ContextWithStaffActor(ctx, head)

	rows, err := repo.ListShipments(bandungCtx, op, false)
	if err != nil || len(rows) != 1 || rows[0].OrderID != bandungOrder {
		t.Fatalf("antrean Bandung bocor: %#v (%v)", rows, err)
	}
	if _, err := repo.GetShipment(bandungCtx, op, medanOrder); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca alamat paket Medan: %v", err)
	}

	foreignDestination := Shipment{
		DeliveryMethod: "SHIP", RecipientName: "Penerima Medan",
		RecipientPhone: "081200000000", ShippingAddress: "Alamat Medan",
	}
	if err := repo.SaveShipmentDestination(bandungCtx, op, medanOrder, foreignDestination); !errors.Is(err, apperror.ErrFailedPrecondition) {
		t.Fatalf("kepala Bandung mengubah tujuan paket Medan: %v", err)
	}
	if err := repo.MarkShipmentSent(bandungCtx, op, medanOrder, "Kurir", "RESI-MEDAN"); !errors.Is(err, apperror.ErrFailedPrecondition) {
		t.Fatalf("kepala Bandung mengirim paket Medan: %v", err)
	}
	if err := repo.MarkShipmentHandedOver(bandungCtx, op, medanOrder, "Penerima", "catatan", "", head); !errors.Is(err, apperror.ErrFailedPrecondition) {
		t.Fatalf("kepala Bandung menyerahkan paket Medan: %v", err)
	}
	if moved, err := repo.MarkShipmentLost(bandungCtx, op, medanOrder, "hilang", head); err != nil || moved {
		t.Fatalf("kepala Bandung menandai paket Medan hilang: moved=%v err=%v", moved, err)
	}

	var status, recipient, tracking string
	if err := pool.QueryRow(ctx, `SELECT status,recipient_name,tracking_number FROM order_fulfilments WHERE order_id=$1`, medanOrder).Scan(&status, &recipient, &tracking); err != nil {
		t.Fatalf("baca paket Medan: %v", err)
	}
	if status != "PENDING" || recipient != "" || tracking != "" {
		t.Fatalf("paket Medan termutasi: status=%q recipient=%q tracking=%q", status, recipient, tracking)
	}

	if err := repo.SaveShipmentDestination(bandungCtx, op, bandungOrder, Shipment{
		DeliveryMethod: "SHIP", RecipientName: "Penerima Bandung",
		RecipientPhone: "081211111111", ShippingAddress: "Alamat Bandung",
	}); err != nil {
		t.Fatalf("kepala Bandung tidak dapat mengisi tujuan sendiri: %v", err)
	}
	if err := repo.MarkShipmentSent(bandungCtx, op, bandungOrder, "Kurir", "RESI-BANDUNG"); err != nil {
		t.Fatalf("kepala Bandung tidak dapat mengirim paket sendiri: %v", err)
	}
	shipment, err := repo.GetShipment(bandungCtx, op, bandungOrder)
	if err != nil || shipment.Status != "SENT" || shipment.TrackingNumber != "RESI-BANDUNG" {
		t.Fatalf("paket Bandung tidak tersimpan: %#v (%v)", shipment, err)
	}

	rows, err = repo.ListShipments(ctx, op, false)
	if err != nil || len(rows) != 2 {
		t.Fatalf("kantor pusat kehilangan antrean paket: %#v (%v)", rows, err)
	}
	if _, err := repo.GetShipment(ctx, op, medanOrder); err != nil {
		t.Fatalf("kantor pusat kehilangan paket Medan: %v", err)
	}
}
