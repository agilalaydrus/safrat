-- name: CreateOrder :one
INSERT INTO orders (
  operator_id, season_id, pilgrim_id, product_id, agent_id, quantity,
  unit_price_idr, total_price_idr, platform_amount_idr, operator_amount_idr, agent_commission_idr
) VALUES ($1, $2, $3, $4, NULLIF($5::text, '')::uuid, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: SetOrderXenditInvoice :exec
UPDATE orders
SET xendit_invoice_id = $2, xendit_invoice_url = $3, updated_at = NOW()
WHERE id = $1;

-- name: MarkOrderPaidByInvoiceID :one
UPDATE orders
SET status = 'PAID', paid_at = NOW(), updated_at = NOW()
WHERE xendit_invoice_id = $1 AND status = 'PENDING'
RETURNING *;

-- name: MarkOrderStatusByInvoiceID :one
UPDATE orders
SET status = $2, updated_at = NOW()
WHERE xendit_invoice_id = $1 AND status = 'PENDING'
RETURNING *;

-- name: GetOrder :one
SELECT o.*, p.full_name AS pilgrim_name, pr.name AS product_name, a.name AS agent_name
FROM orders o
JOIN pilgrims p ON p.id = o.pilgrim_id
JOIN products pr ON pr.id = o.product_id
LEFT JOIN agents a ON a.id = o.agent_id
WHERE o.id = $1 AND o.operator_id = $2;

-- name: ListOrders :many
SELECT o.*, p.full_name AS pilgrim_name, pr.name AS product_name, a.name AS agent_name
FROM orders o
JOIN pilgrims p ON p.id = o.pilgrim_id
JOIN products pr ON pr.id = o.product_id
LEFT JOIN agents a ON a.id = o.agent_id
WHERE o.operator_id = $1 AND o.season_id = $2
ORDER BY o.created_at DESC;

-- name: ListAgentPayouts :many
-- Lifetime, not season-scoped — agents aren't a per-season concept (see
-- AgentService), so neither is what's owed to them.
SELECT a.id AS agent_id, a.name AS agent_name,
       COALESCE(SUM(o.agent_commission_idr) FILTER (WHERE o.status = 'PAID'), 0)::bigint AS total_commission_idr,
       COUNT(o.id) FILTER (WHERE o.status = 'PAID')::int AS paid_order_count
FROM agents a
LEFT JOIN orders o ON o.agent_id = a.id
WHERE a.operator_id = $1
GROUP BY a.id, a.name
ORDER BY total_commission_idr DESC;
