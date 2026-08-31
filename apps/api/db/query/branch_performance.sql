-- name: ListBranchPerformance :many
-- The branch predicate lives in this query, rather than at the RPC boundary:
-- a branch head must never be able to request another branch's report by ID.
SELECT
  b.id AS branch_id,
  b.name AS branch_name,
  b.city AS branch_city,
  b.target_pilgrims,
  b.target_revenue_idr,
  COALESCE(pilgrims.pilgrim_count, 0)::int AS pilgrim_count,
  COALESCE(pilgrims.paid_count, 0)::int AS paid_count,
  COALESCE(pilgrims.documents_ready_count, 0)::int AS documents_ready_count,
  COALESCE(agents.agent_count, 0)::int AS agent_count,
  COALESCE(revenue.revenue_idr, 0)::bigint AS revenue_idr
FROM branches b
LEFT JOIN LATERAL (
  SELECT
    COUNT(*) FILTER (WHERE p.status = 'ACTIVE' AND NOT p.is_substituted) AS pilgrim_count,
    COUNT(*) FILTER (WHERE p.status = 'ACTIVE' AND NOT p.is_substituted AND p.payment_status = 'PAID') AS paid_count,
    COUNT(*) FILTER (
      WHERE p.status = 'ACTIVE' AND NOT p.is_substituted
        AND p.documents_passport AND p.documents_photo AND p.documents_vaccine
    ) AS documents_ready_count
  FROM pilgrims p
  WHERE p.operator_id = b.operator_id AND p.season_id = $2 AND p.branch_id = b.id
) pilgrims ON TRUE
LEFT JOIN LATERAL (
  SELECT COUNT(*) AS agent_count
  FROM agents a
  WHERE a.operator_id = b.operator_id AND a.branch_id = b.id AND a.is_active
) agents ON TRUE
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(op.net_paid_idr), 0)::bigint AS revenue_idr
  FROM orders o
  JOIN order_payments op ON op.order_id = o.id
  WHERE o.operator_id = b.operator_id AND o.season_id = $2 AND o.branch_id = b.id
) revenue ON TRUE
WHERE b.operator_id = $1 AND b.is_active
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR b.id = sqlc.narg(branch_scope)::uuid)
ORDER BY b.name ASC;

-- name: ListBranchPerformanceTrends :many
WITH months AS (
  SELECT generate_series(
    date_trunc('month', CURRENT_DATE) - INTERVAL '11 months',
    date_trunc('month', CURRENT_DATE),
    INTERVAL '1 month'
  )::date AS month
), revenue AS (
  SELECT o.branch_id, date_trunc('month', o.paid_at)::date AS month,
    COALESCE(SUM(op.net_paid_idr), 0)::bigint AS revenue_idr
  FROM orders o
  JOIN order_payments op ON op.order_id = o.id
  WHERE o.operator_id = $1 AND o.season_id = $2 AND o.branch_id IS NOT NULL
    AND o.paid_at >= date_trunc('month', CURRENT_DATE) - INTERVAL '11 months'
  GROUP BY o.branch_id, date_trunc('month', o.paid_at)::date
), pilgrims AS (
  SELECT p.branch_id, date_trunc('month', p.created_at)::date AS month,
    COUNT(*)::int AS pilgrim_count
  FROM pilgrims p
  WHERE p.operator_id = $1 AND p.season_id = $2 AND p.branch_id IS NOT NULL
    AND NOT p.is_substituted
    AND p.created_at >= date_trunc('month', CURRENT_DATE) - INTERVAL '11 months'
  GROUP BY p.branch_id, date_trunc('month', p.created_at)::date
)
SELECT b.id AS branch_id, m.month,
  COALESCE(r.revenue_idr, 0)::bigint AS revenue_idr,
  COALESCE(p.pilgrim_count, 0)::int AS pilgrim_count
FROM branches b
CROSS JOIN months m
LEFT JOIN revenue r ON r.branch_id = b.id AND r.month = m.month
LEFT JOIN pilgrims p ON p.branch_id = b.id AND p.month = m.month
WHERE b.operator_id = $1 AND b.is_active
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR b.id = sqlc.narg(branch_scope)::uuid)
ORDER BY b.id, m.month;
