-- name: CreateOrder :one
-- ON CONFLICT DO NOTHING rather than letting the unique violation raise: the
-- caller recovers the existing order by reading it back, and inside a
-- transaction a failed statement would poison that read.
INSERT INTO orders (
  operator_id, season_id, pilgrim_id, product_id, agent_id, quantity,
  unit_price_idr, total_price_idr, platform_amount_idr, operator_amount_idr,
  agent_commission_idr, idempotency_key, placed_by_agent_id, buyer_agent_id,
  buyer_kind, base_price_idr, operator_markup_idr, agent_markup_idr,
  destination, digital_spend_counted_on
) VALUES (
  sqlc.arg(operator_id), sqlc.arg(season_id), sqlc.narg(pilgrim_id),
  sqlc.arg(product_id), NULLIF(sqlc.arg(agent_id)::text, '')::uuid,
  sqlc.arg(quantity), sqlc.arg(unit_price_idr), sqlc.arg(total_price_idr),
  sqlc.arg(platform_amount_idr), sqlc.arg(operator_amount_idr),
  sqlc.arg(agent_commission_idr), sqlc.arg(idempotency_key),
  NULLIF(sqlc.arg(placed_by_agent_id)::text, '')::uuid,
  sqlc.narg(buyer_agent_id), sqlc.arg(buyer_kind),
  sqlc.arg(base_price_idr), sqlc.arg(operator_markup_idr),
  sqlc.arg(agent_markup_idr), sqlc.arg(destination),
  sqlc.narg(digital_spend_counted_on)
)
ON CONFLICT (operator_id, idempotency_key) WHERE idempotency_key <> '' DO NOTHING
RETURNING *;

-- name: GetOrderByIdempotencyKey :one
SELECT o.*, COALESCE(p.full_name, '') AS pilgrim_name,
       COALESCE(p.full_name, buyer.name, '') AS buyer_name,
       pr.name AS product_name, a.name AS agent_name
FROM orders o
LEFT JOIN pilgrims p ON p.id = o.pilgrim_id
LEFT JOIN agents buyer ON buyer.id = o.buyer_agent_id
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
SELECT o.*, COALESCE(p.full_name, '') AS pilgrim_name,
       COALESCE(p.full_name, buyer.name, '') AS buyer_name,
       pr.name AS product_name, a.name AS agent_name
FROM orders o
LEFT JOIN pilgrims p ON p.id = o.pilgrim_id
LEFT JOIN agents buyer ON buyer.id = o.buyer_agent_id
JOIN products pr ON pr.id = o.product_id
LEFT JOIN agents a ON a.id = o.agent_id
WHERE o.id = $1 AND o.operator_id = $2;

-- name: ListOrders :many
SELECT o.*, COALESCE(p.full_name, '') AS pilgrim_name,
       COALESCE(p.full_name, buyer.name, '') AS buyer_name,
       pr.name AS product_name, a.name AS agent_name
FROM orders o
LEFT JOIN pilgrims p ON p.id = o.pilgrim_id
LEFT JOIN agents buyer ON buyer.id = o.buyer_agent_id
JOIN products pr ON pr.id = o.product_id
LEFT JOIN agents a ON a.id = o.agent_id
WHERE o.operator_id = $1 AND o.season_id = $2
ORDER BY o.created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListOrdersForBuyerAgent :many
SELECT o.*, ''::text AS pilgrim_name, buyer.name AS buyer_name,
       pr.name AS product_name, a.name AS agent_name
FROM orders o
JOIN agents buyer ON buyer.id = o.buyer_agent_id
JOIN products pr ON pr.id = o.product_id
LEFT JOIN agents a ON a.id = o.agent_id
WHERE o.operator_id = $1 AND o.season_id = $2 AND o.buyer_agent_id = $3
ORDER BY o.created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountOrdersForBuyerAgent :one
SELECT COUNT(*) FROM orders
WHERE operator_id = $1 AND season_id = $2 AND buyer_agent_id = $3;

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
LEFT JOIN order_payments op ON op.pilgrim_id = p.id AND op.agent_id = $2
WHERE p.operator_id = $1
GROUP BY p.id, p.full_name
ORDER BY MAX(op.created_at) DESC;

-- name: ListTransactionsForPilgrim :many
-- The jamaah's own history. Every order they ever made, including the ones
-- that were refunded — a refund is something they need to see, not something
-- that quietly removes the transaction from view.
SELECT
  o.id, o.quantity, o.total_price_idr, o.status, o.created_at, o.paid_at,
  o.xendit_invoice_url, o.receipt_number,
  op.name AS operator_name,
  pr.name AS product_name,
  COALESCE(r.amount_idr, 0)::bigint AS refunded_idr,
  r.created_at AS refunded_at,
  COALESCE(r.reason, '') AS refund_reason
FROM orders o
JOIN products pr ON pr.id = o.product_id
JOIN operators op ON op.id = o.operator_id
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

-- name: ListOrdersAwaitingSettlement :many
-- Orders still waiting on the gateway, for the poller that makes a dropped
-- webhook survivable.
--
-- Only orders old enough that a webhook would already have arrived, so the
-- poller does not race the notification for every checkout. Bounded on the far
-- side too: an invoice the gateway never resolved in a week is not going to
-- resolve now.
--
-- And bounded in frequency. The payment window is a day, so without a backoff
-- an abandoned checkout would be asked about every two minutes for that whole
-- day. Payment usually happens in the first minutes, so attention is spent
-- there; after two hours an order is probably abandoned and is checked hourly
-- until its invoice expires. Exhausting the gateway's rate limit would stop
-- settlement for everybody — a worse loss than the dropped webhook this exists
-- to catch.
SELECT id, xendit_invoice_id
FROM orders
WHERE status = 'PENDING'
  AND xendit_invoice_id IS NOT NULL
  AND xendit_invoice_id <> ''
  AND created_at < NOW() - make_interval(mins => $1::int)
  AND created_at > NOW() - INTERVAL '7 days'
  AND (
    last_gateway_check_at IS NULL
    OR last_gateway_check_at < NOW() - (
      CASE
        WHEN created_at > NOW() - INTERVAL '15 minutes' THEN INTERVAL '2 minutes'
        WHEN created_at > NOW() - INTERVAL '2 hours'    THEN INTERVAL '10 minutes'
        ELSE INTERVAL '1 hour'
      END
    )
  )
ORDER BY last_gateway_check_at ASC NULLS FIRST
LIMIT $2;

-- name: MarkOrdersGatewayChecked :exec
-- Recorded whatever the answer was, including none: an order the gateway could
-- not be reached about must still wait its turn, or one unreachable invoice
-- would monopolise every sweep.
UPDATE orders SET last_gateway_check_at = NOW() WHERE id = ANY($1::uuid[]);

-- name: ResolveHeldOrderToPaid :one
-- The operator attests the discrepancy was settled another way. Only HELD
-- moves, so a double-click cannot settle twice.
UPDATE orders
SET status = 'PAID', paid_at = NOW(), updated_at = NOW()
WHERE id = $1 AND operator_id = $2 AND status = 'HELD'
RETURNING *;

-- name: ResolveHeldOrderToFailed :one
-- The transaction is abandoned; the money goes back outside the system.
-- held_reason is kept, not cleared: why it was held is part of the record.
UPDATE orders
SET status = 'FAILED', updated_at = NOW()
WHERE id = $1 AND operator_id = $2 AND status = 'HELD'
RETURNING *;

-- name: GetOrderByIdempotencyKeyAny :one
-- Unscoped by operator, for the supplier callback path: it is authenticated by
-- the supplier's own token and has no operator in hand — the operator is what
-- it is looking up. Every other order read is tenant-scoped and must stay so.
SELECT * FROM orders WHERE id = $1;

-- name: ConsumeDailyDigitalSpend :exec
-- Increments the day's total in place. The row's CHECK is evaluated against
-- the incremented value under the lock the UPDATE already holds, so two
-- concurrent purchases serialise and the second is refused by the database.
-- Reading a total and then deciding would let both through.
INSERT INTO daily_digital_spend (buyer_kind, buyer_id, spend_date, total_idr)
VALUES (sqlc.arg(buyer_kind), sqlc.arg(buyer_id), sqlc.arg(spend_date), sqlc.arg(amount_idr))
ON CONFLICT (buyer_kind, buyer_id, spend_date)
DO UPDATE SET total_idr = daily_digital_spend.total_idr + EXCLUDED.total_idr;

-- name: ReleaseOrderDigitalSpend :exec
-- Gives an order's headroom back when it stops holding value, and un-stamps it
-- in the same statement.
--
-- One statement on purpose. Clearing the stamp and decrementing the total as
-- two round trips can leave an order un-stamped with its total still consumed
-- if anything fails between them, and that headroom is then unreachable for
-- the rest of the day.
--
-- Idempotent by construction: the stamp is the guard. A second call finds no
-- prior row, so the decrement matches nothing and the release cannot fire
-- twice across the three settlement paths and the sweep.
--
-- GREATEST guards the floor. A release larger than the day holds would
-- otherwise hit the non-negative constraint and strand itself.
WITH prior AS (
  SELECT COALESCE(pilgrim_id, buyer_agent_id) AS buyer_id,
         buyer_kind,
         digital_spend_counted_on AS spend_date,
         total_price_idr
  FROM orders o
  WHERE o.id = sqlc.arg(order_id) AND o.digital_spend_counted_on IS NOT NULL
  FOR UPDATE
), cleared AS (
  UPDATE orders u SET digital_spend_counted_on = NULL
  WHERE u.id = sqlc.arg(order_id) AND u.digital_spend_counted_on IS NOT NULL
)
UPDATE daily_digital_spend d
SET total_idr = GREATEST(0, d.total_idr - prior.total_price_idr)
FROM prior
WHERE d.buyer_kind = prior.buyer_kind
  AND d.buyer_id = prior.buyer_id
  AND d.spend_date = prior.spend_date;

-- name: GetDailyDigitalSpend :one
SELECT total_idr, limit_idr FROM daily_digital_spend
WHERE buyer_kind = sqlc.arg(buyer_kind) AND buyer_id = sqlc.arg(buyer_id)
  AND spend_date = sqlc.arg(spend_date);

-- name: PurgeDailyDigitalSpend :one
-- Runs the definer function rather than deleting directly: the application
-- role deliberately holds no DELETE on daily_digital_spend, because removing a
-- row hands an account its whole daily limit back.
SELECT purge_daily_digital_spend(sqlc.arg(keep_days)::int) AS removed;
