-- name: GetOperatorEntitlement :one
SELECT
  COALESCE(po.max_pilgrims, pl.max_pilgrims) AS max_pilgrims,
  COALESCE(po.max_branches, pl.max_branches) AS max_branches,
  pl.feature_flags || COALESCE(po.feature_flag_overrides, '{}'::jsonb) AS feature_flags
FROM operators o
JOIN plan_limits pl ON pl.plan = o.plan
LEFT JOIN plan_overrides po ON po.operator_id = o.id
  AND (po.expires_at IS NULL OR po.expires_at > NOW())
WHERE o.id = $1;

-- name: GetOperatorEntitlementUsage :one
SELECT
  (SELECT COUNT(*)::int FROM pilgrims p WHERE p.operator_id = $1) AS pilgrim_count,
  (SELECT COUNT(*)::int FROM branches b WHERE b.operator_id = $1 AND b.is_active) AS active_branch_count;
