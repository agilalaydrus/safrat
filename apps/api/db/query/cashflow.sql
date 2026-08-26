-- name: CreateVendorPayment :one
INSERT INTO vendor_payments (operator_id, season_id, vendor_name, category, description, amount_idr, due_date)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListVendorPayments :many
SELECT * FROM vendor_payments
WHERE operator_id = $1 AND season_id = $2
ORDER BY due_date ASC;

-- name: UpdateVendorPaymentStatus :one
UPDATE vendor_payments
SET status = $3, paid_at = CASE WHEN $3 = 'PAID' THEN NOW() ELSE NULL END
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteVendorPayment :exec
DELETE FROM vendor_payments WHERE id = $1 AND operator_id = $2;

-- name: GetSeasonPaidTotal :one
-- Total collected from pilgrims — the actual payment ledger (orders net of
-- refunds), not a cached running-balance column.
SELECT COALESCE(SUM(o.net_paid_idr), 0)::bigint AS total_collected_idr
FROM order_payments o
JOIN pilgrims p ON p.id = o.pilgrim_id
WHERE o.operator_id = $1 AND o.season_id = $2 AND p.status = 'ACTIVE';

-- name: GetVendorCommitmentSummary :one
SELECT
  COALESCE(SUM(amount_idr) FILTER (WHERE status != 'CANCELLED'), 0)::bigint AS total_committed_idr,
  COALESCE(SUM(amount_idr) FILTER (WHERE status = 'PAID'), 0)::bigint AS total_paid_out_idr,
  COALESCE(SUM(amount_idr) FILTER (WHERE status IN ('PENDING','OVERDUE')), 0)::bigint AS total_outstanding_idr,
  COALESCE(SUM(amount_idr) FILTER (WHERE status = 'OVERDUE'), 0)::bigint AS total_overdue_idr,
  COALESCE(SUM(amount_idr) FILTER (
    WHERE status IN ('PENDING','OVERDUE') AND due_date BETWEEN CURRENT_DATE AND CURRENT_DATE + 30
  ), 0)::bigint AS due_next_30_days_idr
FROM vendor_payments
WHERE operator_id = $1 AND season_id = $2;

-- name: CountUnpaidPilgrims :one
SELECT COUNT(*) FROM pilgrims
WHERE operator_id = $1 AND season_id = $2 AND status = 'ACTIVE' AND payment_status != 'PAID';

-- name: GetMonthlyProjection :many
SELECT
  DATE_TRUNC('month', due_date)::DATE AS month,
  SUM(amount_idr)::bigint AS vendor_obligations_idr,
  COUNT(id) AS payment_count
FROM vendor_payments
WHERE operator_id = $1
  AND season_id   = $2
  AND status != 'CANCELLED'
GROUP BY 1
ORDER BY 1;

-- name: MarkOverdueVendorPayments :exec
-- Worker sweep — flips PENDING payments whose due_date has passed to OVERDUE.
UPDATE vendor_payments
SET status = 'OVERDUE'
WHERE status = 'PENDING' AND due_date < CURRENT_DATE;
