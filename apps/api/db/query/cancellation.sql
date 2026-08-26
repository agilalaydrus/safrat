-- name: CreateCancellationPolicy :one
INSERT INTO cancellation_policies (operator_id, season_id, name, min_days, refund_pct, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListCancellationPolicies :many
SELECT * FROM cancellation_policies
WHERE operator_id = $1 AND season_id = $2
ORDER BY sort_order ASC;

-- name: DeleteCancellationPolicy :exec
DELETE FROM cancellation_policies WHERE id = $1 AND operator_id = $2;

-- name: GetMatchingPolicy :one
-- First tier (by sort_order) where min_days <= days_before wins — the
-- operator is expected to sort the largest min_days first (sort_order
-- ascending = priority descending on min_days) so a cancellation far out
-- matches the generous tier, not a later stricter one.
SELECT * FROM cancellation_policies
WHERE season_id = $1
  AND min_days <= $2
ORDER BY sort_order ASC
LIMIT 1;

-- name: GetPilgrimPaidTotal :one
-- The actual source of truth for "how much has this pilgrim paid" — computed
-- from orders and refunds, not a cached running-balance column that nothing
-- else keeps in sync.
--
-- Net of refunds, and that is load-bearing: this figure is multiplied by the
-- cancellation policy's refund percentage. Counting a refunded amount as still
-- paid would make the operator return the same money twice. order_payments
-- already reports zero for orders that were never paid, so there is no status
-- filter here to get wrong.
SELECT COALESCE(SUM(net_paid_idr), 0)::bigint AS total_paid_idr
FROM order_payments
WHERE pilgrim_id = $1;

-- name: CreateCancellation :one
-- Immutable. Called inside a transaction alongside MarkPilgrimCancelled.
INSERT INTO pilgrim_cancellations (
  pilgrim_id, operator_id, season_id, reason, days_before,
  refund_pct, refund_amount_idr, total_paid_idr, cancelled_by, policy_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: MarkPilgrimCancelled :exec
UPDATE pilgrims SET status = 'CANCELLED' WHERE id = $1 AND operator_id = $2;

-- name: GetPilgrimCancellation :one
SELECT * FROM pilgrim_cancellations WHERE pilgrim_id = $1 AND operator_id = $2;

-- name: ListCancellations :many
SELECT pc.*, p.full_name AS pilgrim_name
FROM pilgrim_cancellations pc
JOIN pilgrims p ON p.id = pc.pilgrim_id
WHERE pc.operator_id = $1 AND pc.season_id = $2
ORDER BY pc.cancelled_at DESC;
