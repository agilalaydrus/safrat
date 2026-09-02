-- name: ListPlatformPlanLimits :many
SELECT plan::text AS plan, max_pilgrims, max_branches, feature_flags, updated_at
FROM plan_limits
ORDER BY array_position(ARRAY['STARTER','GROWTH','PRO'], plan::text);

-- name: GetPlatformPlanLimit :one
SELECT plan::text AS plan, max_pilgrims, max_branches, feature_flags, updated_at
FROM plan_limits
WHERE plan = sqlc.arg(plan)::plan;

-- name: ListPlatformPlanOverrides :many
SELECT po.operator_id, o.name AS operator_name, o.plan::text AS plan,
       po.max_pilgrims, po.max_branches, po.feature_flag_overrides,
       po.note, po.expires_at, po.updated_by, po.updated_at
FROM plan_overrides po
JOIN operators o ON o.id = po.operator_id
WHERE sqlc.arg(include_expired)::boolean
   OR po.expires_at IS NULL OR po.expires_at > NOW()
ORDER BY po.updated_at DESC, o.name;

-- name: GetPlatformPlanOverride :one
SELECT po.operator_id, o.name AS operator_name, o.plan::text AS plan,
       po.max_pilgrims, po.max_branches, po.feature_flag_overrides,
       po.note, po.expires_at, po.updated_by, po.updated_at
FROM plan_overrides po
JOIN operators o ON o.id = po.operator_id
WHERE po.operator_id = sqlc.arg(operator_id);

-- name: ListTenantsAffectedByPlanLimit :many
WITH usage AS (
  SELECT o.id AS operator_id,
         (SELECT COUNT(*)::int FROM pilgrims p
          WHERE p.operator_id = o.id AND NOT p.is_substituted) AS pilgrim_count,
         (SELECT COUNT(*)::int FROM branches b
          WHERE b.operator_id = o.id AND b.is_active) AS active_branch_count
  FROM operators o
  WHERE o.plan = sqlc.arg(plan)::plan
)
SELECT o.id AS operator_id, o.name, u.pilgrim_count, u.active_branch_count,
       COALESCE(po.max_pilgrims, current_limit.max_pilgrims) AS current_max_pilgrims,
       COALESCE(po.max_branches, current_limit.max_branches) AS current_max_branches
FROM operators o
JOIN usage u ON u.operator_id = o.id
JOIN plan_limits current_limit ON current_limit.plan = o.plan
LEFT JOIN plan_overrides po ON po.operator_id = o.id
  AND (po.expires_at IS NULL OR po.expires_at > NOW())
WHERE o.plan = sqlc.arg(plan)::plan
  AND (
    (po.max_pilgrims IS NULL
      AND sqlc.narg(max_pilgrims)::integer IS NOT NULL
      AND u.pilgrim_count > sqlc.narg(max_pilgrims)::integer)
    OR
    (po.max_branches IS NULL
      AND sqlc.narg(max_branches)::integer IS NOT NULL
      AND u.active_branch_count > sqlc.narg(max_branches)::integer)
    OR
    (NOT (po.feature_flag_overrides ? 'branches')
      AND NOT sqlc.arg(branches_enabled)::boolean
      AND u.active_branch_count > 0)
  )
ORDER BY GREATEST(u.pilgrim_count, u.active_branch_count) DESC, o.name;

-- name: ExpirePlatformPlanOverrides :one
WITH expired AS (
  DELETE FROM plan_overrides
  WHERE expires_at IS NOT NULL AND expires_at <= NOW()
  RETURNING operator_id, note, expires_at, updated_at
), logged AS (
  INSERT INTO audit_logs (
    operator_id, user_id, action, entity_type, entity_id, metadata
  )
  SELECT operator_id, 'system:plan-override-expiry', 'plan_override_expired',
         'plan_override', operator_id::text,
         jsonb_build_object(
           'message', 'override paket berakhir otomatis',
           'previous_note', note,
           'expired_at', expires_at,
           'idempotency_key', operator_id::text || ':' || updated_at::text
         )
  FROM expired
  RETURNING 1
)
SELECT COUNT(*)::int AS expired_count FROM logged;
