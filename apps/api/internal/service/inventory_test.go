package service

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Receiving a PO must move stock and the PO's own status together — a line
// marked received while the shelf still shows the old count would send a
// warehouse operator to look for koper that were never actually delivered.
// This proves ReceivePurchaseOrderItem keeps them in step, including the
// PARTIAL -> RECEIVED transition.
func TestReceivePurchaseOrderItemMovesStockAndRollsStatusIntegration(t *testing.T) {
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

	operatorID, orgID := uuid.NewString(), "inv-"+uuid.NewString()
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Gudang Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "inv-"+operatorID[:8])
	t.Cleanup(func() { exec(`DELETE FROM operators WHERE id = $1`, operatorID) })

	queries := db.New(pool)
	inventoryService := NewInventoryService(
		repository.NewOperatorRepository(queries),
		repository.NewInventoryRepository(queries, pool),
	)

	item, err := inventoryService.CreateItem(ctx, orgID, &hajjv1.CreateInventoryItemRequest{
		Sku: "KOPER-01", Name: "Koper Umrah 24 inci", Unit: "pcs", MinStock: 10, MaxStock: 200, UnitCostIdr: 350_000,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if item.Stock != 0 {
		t.Fatalf("stock awal = %d, mau 0", item.Stock)
	}

	po, err := inventoryService.CreatePurchaseOrder(ctx, orgID, &hajjv1.CreatePurchaseOrderRequest{PoNumber: "PO-001", VendorName: "CV Koper Jaya"})
	if err != nil {
		t.Fatalf("CreatePurchaseOrder: %v", err)
	}
	if err := inventoryService.AddPurchaseOrderItem(ctx, orgID, &hajjv1.AddPurchaseOrderItemRequest{
		PoId: po.Id, ItemId: item.Id, QuantityOrdered: 100, UnitCostIdr: 340_000,
	}); err != nil {
		t.Fatalf("AddPurchaseOrderItem: %v", err)
	}
	items, err := inventoryService.ListPurchaseOrderItems(ctx, orgID, &hajjv1.ListPurchaseOrderItemsRequest{PoId: po.Id})
	if err != nil || len(items.Items) != 1 {
		t.Fatalf("ListPurchaseOrderItems: %v / %+v", err, items)
	}
	poItemID := items.Items[0].Id

	// Partial delivery: 40 of 100. Stock must reflect exactly 40, and the PO
	// must read PARTIAL, not RECEIVED.
	received, err := inventoryService.ReceivePurchaseOrderItem(ctx, orgID, "tester", &hajjv1.ReceivePurchaseOrderItemRequest{PoItemId: poItemID, Quantity: 40})
	if err != nil {
		t.Fatalf("ReceivePurchaseOrderItem (partial): %v", err)
	}
	if received.QuantityReceived != 40 {
		t.Fatalf("quantity_received = %d, mau 40", received.QuantityReceived)
	}
	itemsAfterPartial, err := inventoryService.ListItems(ctx, orgID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if itemsAfterPartial.Items[0].Stock != 40 {
		t.Fatalf("stock setelah partial = %d, mau 40", itemsAfterPartial.Items[0].Stock)
	}
	orders, err := inventoryService.ListPurchaseOrders(ctx, orgID)
	if err != nil || orders.Orders[0].Status != "PARTIAL" {
		t.Fatalf("status PO setelah partial = %v (err=%v), mau PARTIAL", orders.Orders, err)
	}

	// Remaining 60 delivered: stock must reach 100, PO must flip to RECEIVED.
	if _, err := inventoryService.ReceivePurchaseOrderItem(ctx, orgID, "tester", &hajjv1.ReceivePurchaseOrderItemRequest{PoItemId: poItemID, Quantity: 60}); err != nil {
		t.Fatalf("ReceivePurchaseOrderItem (sisa): %v", err)
	}
	itemsFinal, err := inventoryService.ListItems(ctx, orgID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if itemsFinal.Items[0].Stock != 100 {
		t.Fatalf("stock akhir = %d, mau 100", itemsFinal.Items[0].Stock)
	}
	ordersFinal, err := inventoryService.ListPurchaseOrders(ctx, orgID)
	if err != nil || ordersFinal.Orders[0].Status != "RECEIVED" {
		t.Fatalf("status PO akhir = %v (err=%v), mau RECEIVED", ordersFinal.Orders, err)
	}

	// Receiving past what was ordered must be refused, not silently overshoot.
	if _, err := inventoryService.ReceivePurchaseOrderItem(ctx, orgID, "tester", &hajjv1.ReceivePurchaseOrderItemRequest{PoItemId: poItemID, Quantity: 1}); err == nil {
		t.Fatalf("mau ditolak: menerima melebihi quantity_ordered semestinya tidak valid")
	}

	// A stock movement below zero must be refused, and must not silently
	// leave the ledger and the running total disagreeing.
	if _, err := inventoryService.AdjustStock(ctx, orgID, "tester", &hajjv1.AdjustStockRequest{
		ItemId: item.Id, MovementType: "OUT", Quantity: 1000, Reason: "uji",
	}); err == nil {
		t.Fatalf("mau ditolak: OUT melebihi stok semestinya tidak valid")
	}
	itemsUnchanged, err := inventoryService.ListItems(ctx, orgID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if itemsUnchanged.Items[0].Stock != 100 {
		t.Fatalf("stock setelah OUT ditolak = %d, mau tetap 100", itemsUnchanged.Items[0].Stock)
	}

	summary, err := inventoryService.GetSummary(ctx, orgID)
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary.ValuationIdr != 100*350_000 {
		t.Fatalf("valuasi = %d, mau %d", summary.ValuationIdr, int64(100*350_000))
	}
}
