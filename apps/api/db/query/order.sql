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

-- name: MarkOrderPaidManually :one
-- Only PENDING -> PAID, same guard as MarkOrderPaidByInvoiceID — an
-- operator retrying a slow click can't double-count a cash sale.
UPDATE orders
SET status = 'PAID', paid_at = NOW(), updated_at = NOW()
WHERE id = $1 AND operator_id = $2 AND status = 'PENDING'
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
-- AgentService), so neither is what's owed to them. total_disbursed_idr
-- comes from the agent_payouts ledger (see migration 039); outstanding is
-- computed by the caller as total_commission_idr - total_disbursed_idr.
SELECT a.id AS agent_id, a.name AS agent_name,
       COALESCE(SUM(o.agent_commission_idr) FILTER (WHERE o.status = 'PAID'), 0)::bigint AS total_commission_idr,
       COUNT(o.id) FILTER (WHERE o.status = 'PAID')::int AS paid_order_count,
       COALESCE(disb.total, 0)::bigint AS total_disbursed_idr
FROM agents a
LEFT JOIN orders o ON o.agent_id = a.id
LEFT JOIN (SELECT agent_id, SUM(amount_idr) AS total FROM agent_payouts GROUP BY agent_id) disb ON disb.agent_id = a.id
WHERE a.operator_id = $1
GROUP BY a.id, a.name, disb.total
ORDER BY total_commission_idr DESC;

-- name: GetAgentPayoutSummary :one
SELECT a.id AS agent_id, a.name AS agent_name,
       COALESCE(SUM(o.agent_commission_idr) FILTER (WHERE o.status = 'PAID'), 0)::bigint AS total_commission_idr,
       COUNT(o.id) FILTER (WHERE o.status = 'PAID')::int AS paid_order_count,
       COALESCE(disb.total, 0)::bigint AS total_disbursed_idr
FROM agents a
LEFT JOIN orders o ON o.agent_id = a.id
LEFT JOIN (SELECT agent_id, SUM(amount_idr) AS total FROM agent_payouts GROUP BY agent_id) disb ON disb.agent_id = a.id
WHERE a.id = $2 AND a.operator_id = $1
GROUP BY a.id, a.name, disb.total;

-- name: RecordAgentPayout :one
INSERT INTO agent_payouts (operator_id, agent_id, amount_idr, note, paid_by_user_id, method, request_id)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7::text, '')::uuid)
RETURNING *;

-- name: ListAgentPayoutHistory :many
SELECT p.id, p.amount_idr, p.note, p.method, p.created_at, u.name AS paid_by_name
FROM agent_payouts p
JOIN "user" u ON u.id = p.paid_by_user_id
WHERE p.agent_id = $1 AND p.operator_id = $2
ORDER BY p.created_at DESC;

-- name: ListOrderCreditsForAgent :many
SELECT o.id, o.agent_commission_idr, o.paid_at, pr.name AS product_name
FROM orders o
JOIN products pr ON pr.id = o.product_id
WHERE o.agent_id = $1 AND o.status = 'PAID'
ORDER BY o.paid_at DESC;

-- name: SumPendingPayoutRequests :one
SELECT COALESCE(SUM(amount_idr), 0)::bigint AS total
FROM agent_payout_requests
WHERE agent_id = $1 AND status = 'PENDING';

-- name: CreatePayoutRequest :one
INSERT INTO agent_payout_requests (operator_id, agent_id, amount_idr, note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPayoutRequest :one
SELECT r.*, a.name AS agent_name
FROM agent_payout_requests r
JOIN agents a ON a.id = r.agent_id
WHERE r.id = $1 AND r.operator_id = $2;

-- name: ListPayoutRequests :many
SELECT r.*, a.name AS agent_name
FROM agent_payout_requests r
JOIN agents a ON a.id = r.agent_id
WHERE r.operator_id = $1 AND r.status = 'PENDING'
  AND (sqlc.narg(agent_id)::uuid IS NULL OR r.agent_id = sqlc.narg(agent_id)::uuid)
ORDER BY r.requested_at ASC;

-- name: ApprovePayoutRequestTx :one
UPDATE agent_payout_requests
SET status = 'APPROVED', resolved_at = NOW(), resolved_by_user_id = $3
WHERE id = $1 AND operator_id = $2 AND status = 'PENDING'
RETURNING *;

-- name: RejectPayoutRequest :one
UPDATE agent_payout_requests
SET status = 'REJECTED', resolution_note = $4, resolved_at = NOW(), resolved_by_user_id = $3
WHERE id = $1 AND operator_id = $2 AND status = 'PENDING'
RETURNING *;
