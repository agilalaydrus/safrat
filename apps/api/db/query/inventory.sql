-- name: CreateInventoryItem :one
INSERT INTO inventory_items (
  operator_id, sku, name, unit, min_stock, max_stock, unit_cost_idr,
  per_pilgrim_qty, per_pilgrim_notes, moq, lead_time_days, vendor_name, rak
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: UpdateInventoryItem :one
UPDATE inventory_items
SET name = $3, unit = $4, min_stock = $5, max_stock = $6, unit_cost_idr = $7,
    per_pilgrim_qty = $8, per_pilgrim_notes = $9, moq = $10, lead_time_days = $11,
    vendor_name = $12, rak = $13
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteInventoryItem :exec
DELETE FROM inventory_items WHERE id = $1 AND operator_id = $2;

-- name: GetInventoryItem :one
SELECT * FROM inventory_items WHERE id = $1 AND operator_id = $2;

-- name: ListInventoryItems :many
SELECT * FROM inventory_items WHERE operator_id = $1 ORDER BY name;

-- name: AdjustInventoryStock :one
UPDATE inventory_items
SET stock = stock + sqlc.arg(delta)::int,
    last_restock_at = CASE WHEN sqlc.arg(delta)::int > 0 THEN NOW() ELSE last_restock_at END
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: CreateStockMovement :one
INSERT INTO inventory_stock_movements (operator_id, item_id, movement_type, quantity, reason, reference, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListStockMovements :many
SELECT * FROM inventory_stock_movements
WHERE operator_id = $1 AND item_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: CreatePurchaseOrder :one
INSERT INTO purchase_orders (operator_id, po_number, vendor_name, eta_date, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdatePurchaseOrderStatus :one
UPDATE purchase_orders SET status = $3 WHERE id = $1 AND operator_id = $2 RETURNING *;

-- name: ListPurchaseOrders :many
SELECT * FROM purchase_orders WHERE operator_id = $1 ORDER BY created_at DESC;

-- name: GetPurchaseOrder :one
SELECT * FROM purchase_orders WHERE id = $1 AND operator_id = $2;

-- name: CreatePurchaseOrderItem :one
INSERT INTO purchase_order_items (operator_id, po_id, item_id, quantity_ordered, unit_cost_idr)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPurchaseOrderItems :many
SELECT poi.*, i.sku, i.name AS item_name, i.unit
FROM purchase_order_items poi
JOIN inventory_items i ON i.id = poi.item_id
WHERE poi.po_id = $1 AND poi.operator_id = $2
ORDER BY poi.created_at;

-- name: ReceivePurchaseOrderItem :one
UPDATE purchase_order_items
SET quantity_received = quantity_received + sqlc.arg(delta)::int
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: PurchaseOrderReceiptTotals :one
SELECT
  COALESCE(SUM(quantity_ordered), 0)::int AS ordered_total,
  COALESCE(SUM(quantity_received), 0)::int AS received_total
FROM purchase_order_items
WHERE po_id = $1 AND operator_id = $2;

-- name: InventoryValuation :one
SELECT COALESCE(SUM(stock * unit_cost_idr), 0)::bigint AS value_idr FROM inventory_items WHERE operator_id = $1;

-- name: InventoryBelowMinimumCount :one
SELECT COUNT(*)::int FROM inventory_items WHERE operator_id = $1 AND stock < min_stock;

-- name: PurchaseOrdersOpenCount :one
SELECT COUNT(*)::int FROM purchase_orders WHERE operator_id = $1 AND status IN ('DRAFT','ORDERED','PARTIAL');

-- name: StockOutQuantitySince :one
SELECT COALESCE(SUM(quantity), 0)::bigint FROM inventory_stock_movements
WHERE operator_id = $1 AND movement_type = 'OUT' AND created_at >= $2;
