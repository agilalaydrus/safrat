-- name: ProfitLossByPeriod :many
-- One row per calendar month, oldest first, over PAID orders whose paid_at
-- falls within the window. Cost is summed only where the product's own
-- supplier_cost_idr is known; orders_missing_cost/revenue_missing_cost_idr
-- carry what that leaves out, so the caller can say so rather than treating
-- an unknown cost as zero.
SELECT
  date_trunc('month', o.paid_at)::date AS period_start,
  SUM(o.total_price_idr)::bigint AS revenue_idr,
  SUM(o.platform_amount_idr)::bigint AS platform_amount_idr,
  SUM(o.agent_commission_idr)::bigint AS agent_commission_idr,
  SUM(CASE WHEN p.supplier_cost_idr IS NOT NULL THEN o.quantity * p.supplier_cost_idr ELSE 0 END)::bigint AS cost_idr,
  SUM(o.quantity)::int AS unit_count,
  COUNT(*) FILTER (WHERE p.supplier_cost_idr IS NULL)::int AS orders_missing_cost,
  COALESCE(SUM(o.total_price_idr) FILTER (WHERE p.supplier_cost_idr IS NULL), 0)::bigint AS revenue_missing_cost_idr
FROM orders o
JOIN products p ON p.id = o.product_id
WHERE o.operator_id = $1 AND o.status = 'PAID' AND o.paid_at >= $2
GROUP BY period_start
ORDER BY period_start;

-- name: ProfitLossByBranch :many
-- head_office (branch_id IS NULL) rows are included as a single group —
-- an operator with no branches has exactly one row here, not zero.
SELECT
  b.id AS branch_id, COALESCE(b.name, '') AS branch_name,
  COALESCE(b.target_revenue_idr, 0)::bigint AS target_revenue_idr,
  SUM(o.total_price_idr)::bigint AS revenue_idr,
  SUM(o.operator_amount_idr)::bigint AS operator_amount_idr,
  SUM(CASE WHEN p.supplier_cost_idr IS NOT NULL THEN o.quantity * p.supplier_cost_idr ELSE 0 END)::bigint AS cost_idr
FROM orders o
JOIN products p ON p.id = o.product_id
LEFT JOIN branches b ON b.id = o.branch_id
WHERE o.operator_id = $1 AND o.status = 'PAID' AND o.paid_at >= $2
GROUP BY b.id, b.name, b.target_revenue_idr
ORDER BY revenue_idr DESC;

-- name: ProfitLossByAgent :many
SELECT
  a.id AS agent_id, a.name AS agent_name,
  SUM(o.total_price_idr)::bigint AS revenue_idr,
  SUM(o.agent_commission_idr)::bigint AS commission_idr,
  COUNT(*)::int AS order_count
FROM orders o
JOIN agents a ON a.id = o.agent_id
WHERE o.operator_id = $1 AND o.status = 'PAID' AND o.paid_at >= $2
GROUP BY a.id, a.name
ORDER BY revenue_idr DESC;

