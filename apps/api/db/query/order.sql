-- name: CreateOrder :one
-- ON CONFLICT DO NOTHING rather than letting the unique violation raise: the
-- caller recovers the existing order by reading it back, and inside a
-- transaction a failed statement would poison that read.
INSERT INTO orders (
  operator_id, season_id, pilgrim_id, product_id, agent_id, quantity,
  unit_price_idr, total_price_idr, platform_amount_idr, operator_amount_idr,
  agent_commission_idr, idempotency_key, placed_by_agent_id
) VALUES ($1, $2, $3, $4, NULLIF($5::text, '')::uuid, $6, $7, $8, $9, $10, $11,
          $12, NULLIF($13::text, '')::uuid)
ON CONFLICT (operator_id, idempotency_key) WHERE idempotency_key <> '' DO NOTHING
RETURNING *;

-- name: GetOrderByIdempotencyKey :one
SELECT o.*, p.full_name AS pilgrim_name, pr.name AS product_name, a.name AS agent_name
FROM orders o
JOIN pilgrims p ON p.id = o.pilgrim_id
JOIN products pr ON pr.id = o.product_id
LEFT JOIN agents a ON a.id = o.agent_id
WHERE o.operator_id = $1 AND o.idempotency_key = $2;

-- name: SetOrderXenditInvoice :exec
UPDATE orders
SET xendit_invoice_id = $2, xendit_invoice_url = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetOrderByInvoiceID :one
SELECT * FROM orders WHERE xendit_invoice_id = $1;

-- name: MarkOrderPaidByInvoiceID :one
-- Records the amount the gateway reported alongside the settlement, so a
-- settled order carries the evidence that the amount was checked, not just the
-- claim that it was.
UPDATE orders
SET status = 'PAID', paid_at = NOW(), paid_amount_idr = $2, updated_at = NOW()
WHERE xendit_invoice_id = $1 AND status = 'PENDING'
RETURNING *;

-- name: HoldOrderByInvoiceID :one
-- Money arrived, but not the amount that was owed. Neither settled nor
-- rejected: rejecting would strand a real payment, and settling would accept
-- an amount nobody agreed to.
UPDATE orders
SET status = 'HELD', paid_amount_idr = $2, held_reason = $3, updated_at = NOW()
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
ORDER BY o.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountOrdersBySeason :one
SELECT COUNT(*) FROM orders WHERE operator_id = $1 AND season_id = $2;

-- name: ListAgentPayouts :many
-- Lifetime, not season-scoped — agents aren't a per-season concept (see
-- AgentService), so neither is what's owed to them. total_disbursed_idr
-- comes from the agent_payouts ledger (see migration 039); outstanding is
-- computed by the caller as total_commission_idr - total_disbursed_idr.
-- Same ledger-backed commission as GetAgentPayoutSummary.
SELECT a.id AS agent_id, a.name AS agent_name,
       COALESCE(st.recognised_idr, 0)::bigint AS total_commission_idr,
       COALESCE(st.settled_idr, 0)::bigint AS settled_commission_idr,
       COALESCE(st.pending_idr, 0)::bigint AS pending_commission_idr,
       COALESCE(ord.paid_count, 0)::int AS paid_order_count,
       COALESCE(disb.total, 0)::bigint AS total_disbursed_idr
FROM agents a
LEFT JOIN agent_commission_state st ON st.agent_id = a.id
LEFT JOIN (SELECT agent_id, COUNT(*) AS paid_count FROM orders WHERE status = 'PAID' GROUP BY agent_id) ord ON ord.agent_id = a.id
LEFT JOIN (SELECT agent_id, SUM(amount_idr) AS total FROM agent_payouts GROUP BY agent_id) disb ON disb.agent_id = a.id
WHERE a.operator_id = $1
ORDER BY total_commission_idr DESC;

-- name: GetAgentPayoutSummary :one
-- Commission comes from the ledger, not from summing orders. Summing orders
-- meant a refund silently changed history: flip an order out of PAID and the
-- agent's earnings moved with no record that a reversal happened. The ledger
-- carries an explicit reversing entry instead, so the balance and the reason
-- for it are both auditable. Order count still comes from orders, because that
-- is what it counts.
--
-- total_commission_idr is everything recognised, pending included, because a
-- pending transaction already counts. settled_commission_idr is the part
-- behind a completed transaction, and is the only figure a payout may draw on
-- — paying out pending commission would advance money for a transaction that
-- may still fail.
SELECT a.id AS agent_id, a.name AS agent_name,
       COALESCE(st.recognised_idr, 0)::bigint AS total_commission_idr,
       COALESCE(st.settled_idr, 0)::bigint AS settled_commission_idr,
       COALESCE(st.pending_idr, 0)::bigint AS pending_commission_idr,
       COALESCE(ord.paid_count, 0)::int AS paid_order_count,
       COALESCE(disb.total, 0)::bigint AS total_disbursed_idr
FROM agents a
LEFT JOIN agent_commission_state st ON st.agent_id = a.id
LEFT JOIN (SELECT agent_id, COUNT(*) AS paid_count FROM orders WHERE status = 'PAID' GROUP BY agent_id) ord ON ord.agent_id = a.id
LEFT JOIN (SELECT agent_id, SUM(amount_idr) AS total FROM agent_payouts GROUP BY agent_id) disb ON disb.agent_id = a.id
WHERE a.id = $2 AND a.operator_id = $1;

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

-- name: ListCommissionEntriesForAgent :many
-- Read from the commission ledger, not from PAID orders.
--
-- Summing orders had a failure that only appeared once refunds existed: a
-- refunded order leaves PAID, so its earning vanished from this list entirely
-- while the agent's balance dropped by the same amount. Money disappeared with
-- no line explaining it. The ledger keeps both the earning and the reversal,
-- so the list explains the balance instead of contradicting it.
SELECT e.id, e.amount_idr, e.kind, e.note, e.created_at,
       COALESCE(pr.name, '') AS product_name
FROM agent_commission_entries e
LEFT JOIN orders o ON o.id = e.order_id
LEFT JOIN products pr ON pr.id = o.product_id
WHERE e.agent_id = $1
ORDER BY e.created_at DESC;

-- name: ListReferredCustomerRecapForAgent :many
-- Per-jamaah money recap for one agent's referrals.
--
-- Paid and refunded come from order_payments, so a refunded order contributes
-- nothing to what is held. Commission comes from the ledger for the same
-- reason it does everywhere else: it is the only place a reversal is recorded.
SELECT
  p.id AS pilgrim_id,
  p.full_name AS pilgrim_name,
  COUNT(op.order_id)::int AS order_count,
  COUNT(*) FILTER (WHERE op.status = 'REFUNDED')::int AS refunded_order_count,
  COALESCE(SUM(op.net_paid_idr), 0)::bigint AS total_paid_idr,
  COALESCE(SUM(op.refunded_idr), 0)::bigint AS refunded_idr,
  COALESCE((
    SELECT SUM(e.amount_idr) FROM agent_commission_entries e
    JOIN orders eo ON eo.id = e.order_id
    WHERE e.agent_id = $2 AND eo.pilgrim_id = p.id
  ), 0)::bigint AS commission_idr,
  MAX(op.created_at)::timestamptz AS last_transaction_at
FROM pilgrims p
JOIN order_payments op ON op.pilgrim_id = p.id AND op.agent_id = $2
WHERE p.operator_id = $1
GROUP BY p.id, p.full_name
ORDER BY MAX(op.created_at) DESC;

-- name: ListTransactionsForPilgrim :many
-- The jamaah's own history. Every order they ever made, including the ones
-- that were refunded — a refund is something they need to see, not something
-- that quietly removes the transaction from view.
SELECT
  o.id, o.quantity, o.total_price_idr, o.status, o.created_at, o.paid_at,
  o.xendit_invoice_url,
  pr.name AS product_name,
  COALESCE(r.amount_idr, 0)::bigint AS refunded_idr,
  r.created_at AS refunded_at,
  COALESCE(r.reason, '') AS refund_reason
FROM orders o
JOIN products pr ON pr.id = o.product_id
LEFT JOIN order_refunds r ON r.order_id = o.id
WHERE o.pilgrim_id = $1
ORDER BY o.created_at DESC;

-- name: GetPilgrimTransactionTotals :one
SELECT
  COALESCE(SUM(net_paid_idr), 0)::bigint AS total_paid_idr,
  COALESCE(SUM(refunded_idr), 0)::bigint AS total_refunded_idr
FROM order_payments
WHERE pilgrim_id = $1;

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
