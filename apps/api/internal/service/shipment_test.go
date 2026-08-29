package service

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type shipmentFixture struct {
	pool       *pgxpool.Pool
	shipments  *ShipmentService
	orgID      string
	operatorID string
	orderID    string
}

func newShipmentFixture(t *testing.T) *shipmentFixture {
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

	operatorID, orgID := uuid.NewString(), "ship-"+uuid.NewString()
	seasonID, productID, pilgrimID, orderID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Kirim Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "ship-"+operatorID[:8])
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
		_ = cleanup.Commit(bg)
	})

	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID)
	// Equipment: a travel's own product, handed over by a person.
	exec(`INSERT INTO products (id, operator_id, season_id, name, category, price_idr, base_price_idr, code)
	      VALUES ($1,$2,$3,'Koper Umrah','EQUIPMENT',500000,400000,$4)`,
		productID, operatorID, seasonID, "KOPER-"+productID[:8])
	exec(`INSERT INTO product_markups (product_id, operator_id, operator_markup_idr, agent_markup_idr)
	      VALUES ($1,$2,100000,0)`, productID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$2,$3,'Jamaah Kirim','P-SHIP','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimID, seasonID, operatorID)
	exec(`INSERT INTO orders (id, operator_id, season_id, pilgrim_id, product_id, quantity,
	        unit_price_idr, total_price_idr, status, paid_at, paid_amount_idr)
	      VALUES ($1,$2,$3,$4,$5,1,500000,500000,'PAID',NOW(),500000)`,
		orderID, operatorID, seasonID, pilgrimID, productID)

	queries := db.New(pool)
	fulfilments := repository.NewFulfilmentRepository(pool)
	shipments := NewShipmentService(repository.NewOperatorRepository(queries), fulfilments, repository.NewAuditRepository(queries), nil)

	// Opened the way a paid order opens one.
	if _, err := fulfilments.Open(ctx, orderID, operatorID, "", "SHIPMENT"); err != nil {
		t.Fatalf("open shipment: %v", err)
	}

	return &shipmentFixture{pool: pool, shipments: shipments, orgID: orgID, operatorID: operatorID, orderID: orderID}
}

// Equipment has no supplier and never will, so it must not appear as a routing
// fault. Before this it did — and a queue that fills with things nobody can fix
// is a queue people stop reading.
func TestEquipmentOpensAsAShipmentNotASupplierFaultIntegration(t *testing.T) {
	f := newShipmentFixture(t)
	ctx := context.Background()

	var kind, status, lastError string
	if err := f.pool.QueryRow(ctx,
		`SELECT kind, status, last_error FROM order_fulfilments WHERE order_id = $1`, f.orderID).
		Scan(&kind, &status, &lastError); err != nil {
		t.Fatalf("read fulfilment: %v", err)
	}
	if kind != "SHIPMENT" {
		t.Fatalf("kind = %q, mau SHIPMENT", kind)
	}
	if status != "PENDING" {
		t.Fatalf("status = %q, mau PENDING — perlengkapan menunggu orang, bukan supplier", status)
	}
	if lastError != "" {
		t.Fatalf("perlengkapan diberi alasan kesalahan %q yang tidak bisa diperbaiki siapa pun", lastError)
	}

	// And it shows up in the queue a person works from.
	list, err := f.shipments.ListShipments(ctx, f.orgID, &hajjv1.ListShipmentsRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Shipments) != 1 || list.Shipments[0].OrderId != f.orderID {
		t.Fatalf("antrean pengiriman = %+v", list.Shipments)
	}
}

// A parcel with no courier or tracking is one nobody can answer a question
// about, and an address is not optional for something being posted.
func TestShipmentCannotBeSentWithoutDestinationOrTrackingIntegration(t *testing.T) {
	f := newShipmentFixture(t)
	ctx := context.Background()

	if _, err := f.shipments.MarkSent(ctx, f.orgID, "staf", &hajjv1.MarkShipmentSentRequest{
		OrderId: f.orderID, Courier: "JNE", TrackingNumber: "JNE1",
	}); err == nil {
		t.Fatal("paket terkirim tanpa tujuan yang lengkap")
	}

	if _, err := f.shipments.SaveDestination(ctx, f.orgID, "staf", &hajjv1.SaveShipmentDestinationRequest{
		OrderId: f.orderID, DeliveryMethod: "SHIP", RecipientName: "", ShippingAddress: "",
	}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("SHIP tanpa penerima/alamat dikembalikan %v", err)
	}
}

// Where a parcel went stops being editable once it has gone. Otherwise the
// answer to "where did this go?" can be rewritten with no trace it differed.
func TestDispatchedShipmentDestinationIsFrozenIntegration(t *testing.T) {
	f := newShipmentFixture(t)
	ctx := context.Background()

	save := func(address string) error {
		_, err := f.shipments.SaveDestination(ctx, f.orgID, "staf", &hajjv1.SaveShipmentDestinationRequest{
			OrderId: f.orderID, DeliveryMethod: "SHIP", RecipientName: "Ahmad",
			RecipientPhone: "081200000000", ShippingAddress: address,
		})
		return err
	}

	if err := save("Jl. Merdeka 1"); err != nil {
		t.Fatalf("simpan tujuan: %v", err)
	}
	// Correcting an address read over the phone is ordinary work.
	if err := save("Jl. Merdeka 2"); err != nil {
		t.Fatalf("koreksi sebelum kirim ditolak: %v", err)
	}

	sent, err := f.shipments.MarkSent(ctx, f.orgID, "staf", &hajjv1.MarkShipmentSentRequest{
		OrderId: f.orderID, Courier: "JNE", TrackingNumber: "JNE-123",
	})
	if err != nil {
		t.Fatalf("tandai terkirim: %v", err)
	}
	if sent.DestinationEditable {
		t.Fatal("paket terkirim masih dilaporkan bisa diubah tujuannya")
	}

	if err := save("Jl. Lain 9"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("mengubah tujuan paket terkirim dikembalikan %v, mau failed precondition", err)
	}

	// Re-sending must not restart a parcel that has already gone.
	if _, err := f.shipments.MarkSent(ctx, f.orgID, "staf", &hajjv1.MarkShipmentSentRequest{
		OrderId: f.orderID, Courier: "SiCepat", TrackingNumber: "SC-9",
	}); err == nil {
		t.Fatal("paket yang sudah berangkat dikirim ulang")
	}
}

// Delivery is evidence, not a status somebody clicked. It needs a name, and it
// cannot be recorded twice — the second write would overwrite who signed.
func TestHandoverNeedsARecipientAndHappensOnceIntegration(t *testing.T) {
	f := newShipmentFixture(t)
	ctx := context.Background()

	if _, err := f.shipments.MarkHandedOver(ctx, f.orgID, "staf", &hajjv1.MarkShipmentHandedOverRequest{
		OrderId: f.orderID, HandoverRecipient: "",
	}); err == nil {
		t.Fatal("serah terima tercatat tanpa nama penerima")
	}

	// A jamaah collecting at the counter never has a dispatch step.
	if _, err := f.shipments.SaveDestination(ctx, f.orgID, "staf", &hajjv1.SaveShipmentDestinationRequest{
		OrderId: f.orderID, DeliveryMethod: "PICKUP", RecipientName: "Ahmad",
	}); err != nil {
		t.Fatalf("ambil sendiri: %v", err)
	}
	done, err := f.shipments.MarkHandedOver(ctx, f.orgID, "staf", &hajjv1.MarkShipmentHandedOverRequest{
		OrderId: f.orderID, HandoverRecipient: "Ahmad bin Ali", HandoverNote: "diambil di kantor",
	})
	if err != nil {
		t.Fatalf("serah terima: %v", err)
	}
	if done.Status != "DELIVERED" || done.HandoverRecipient != "Ahmad bin Ali" || done.DeliveredAt == nil {
		t.Fatalf("bukti serah terima tidak lengkap: %+v", done)
	}

	if _, err := f.shipments.MarkHandedOver(ctx, f.orgID, "staf", &hajjv1.MarkShipmentHandedOverRequest{
		OrderId: f.orderID, HandoverRecipient: "Orang Lain",
	}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("serah terima kedua dikembalikan %v — penerima asli bisa tertimpa", err)
	}

	// And it leaves the working queue.
	list, err := f.shipments.ListShipments(ctx, f.orgID, &hajjv1.ListShipmentsRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Shipments) != 0 {
		t.Fatal("paket yang sudah diserahkan masih di antrean kerja")
	}
}

// A key recorded for an object that does not exist would read as evidence of
// something that never happened — worse than no photo, because the row would
// claim the handover was documented. With storage unconfigured the only honest
// answer is to refuse the key, while still letting the handover be recorded by
// name.
func TestHandoverProofKeyIsNeverTakenOnTrustIntegration(t *testing.T) {
	f := newShipmentFixture(t)
	ctx := context.Background()

	if _, err := f.shipments.SaveDestination(ctx, f.orgID, "staf", &hajjv1.SaveShipmentDestinationRequest{
		OrderId: f.orderID, DeliveryMethod: "PICKUP", RecipientName: "Ahmad",
	}); err != nil {
		t.Fatalf("tujuan: %v", err)
	}

	// This fixture has no object storage, so any key is unverifiable.
	if _, err := f.shipments.MarkHandedOver(ctx, f.orgID, "staf", &hajjv1.MarkShipmentHandedOverRequest{
		OrderId: f.orderID, HandoverRecipient: "Ahmad", HandoverProofKey: "handover/palsu/photo.jpg",
	}); err == nil {
		t.Fatal("kunci foto yang tidak dapat diverifikasi diterima")
	}

	// Without a key it still works: recording a handover by name is worth doing
	// on its own, and photo storage being unconfigured must not block delivery.
	done, err := f.shipments.MarkHandedOver(ctx, f.orgID, "staf", &hajjv1.MarkShipmentHandedOverRequest{
		OrderId: f.orderID, HandoverRecipient: "Ahmad bin Ali",
	})
	if err != nil {
		t.Fatalf("serah terima tanpa foto ditolak: %v", err)
	}
	if done.HasHandoverProof {
		t.Fatal("dilaporkan punya bukti foto padahal tidak ada")
	}

	// And the view path answers empty rather than erroring, so a screen can ask
	// without knowing in advance.
	view, err := f.shipments.GetProofURL(ctx, f.orgID, &hajjv1.GetHandoverProofUrlRequest{OrderId: f.orderID})
	if err != nil {
		t.Fatalf("get proof url: %v", err)
	}
	if view.ViewUrl != "" {
		t.Fatalf("URL diberikan padahal tidak ada foto: %q", view.ViewUrl)
	}
}
