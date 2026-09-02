-- name: CreateCRMLead :one
INSERT INTO crm_leads (
  operator_id, branch_id, full_name, phone, email, source, campaign,
  season_id, product_id, assignee_user_id, pax, estimated_value_idr,
  next_action, next_follow_up_at, created_by_user_id, idempotency_key,
  request_fingerprint
) VALUES (
  sqlc.arg(operator_id), sqlc.narg(branch_scope)::uuid, sqlc.arg(full_name),
  sqlc.arg(phone), sqlc.arg(email), sqlc.arg(source), sqlc.arg(campaign),
  sqlc.narg(season_id)::uuid, sqlc.narg(product_id)::uuid,
  sqlc.narg(assignee_user_id)::text, sqlc.arg(pax),
  sqlc.arg(estimated_value_idr), sqlc.arg(next_action),
  sqlc.narg(next_follow_up_at)::timestamptz, sqlc.arg(created_by_user_id),
  sqlc.arg(idempotency_key), sqlc.arg(request_fingerprint)
)
RETURNING *;

-- name: LockCRMIdempotencyKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(
  sqlc.arg(operator_id)::uuid::text || ':' || sqlc.arg(idempotency_key)::text,
  0
));

-- name: GetCRMLeadByIdempotency :one
SELECT * FROM crm_leads
WHERE operator_id = sqlc.arg(operator_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid);

-- name: GetCRMLead :one
SELECT l.*, COALESCE(s.name, '') AS season_name,
       COALESCE(p.name, '') AS product_name,
       COALESCE(u.name, '') AS assignee_name
FROM crm_leads l
LEFT JOIN seasons s ON s.id = l.season_id
LEFT JOIN products p ON p.id = l.product_id
LEFT JOIN "user" u ON u.id = l.assignee_user_id
WHERE l.id = sqlc.arg(id)
  AND l.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR l.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: ListCRMLeads :many
SELECT l.*, COALESCE(s.name, '') AS season_name,
       COALESCE(p.name, '') AS product_name,
       COALESCE(u.name, '') AS assignee_name,
       COUNT(*) OVER() AS total_count
FROM crm_leads l
LEFT JOIN seasons s ON s.id = l.season_id
LEFT JOIN products p ON p.id = l.product_id
LEFT JOIN "user" u ON u.id = l.assignee_user_id
WHERE l.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR l.branch_id = sqlc.narg(branch_scope)::uuid)
  AND (sqlc.narg(stage)::text IS NULL OR l.stage = sqlc.narg(stage)::text)
  AND (sqlc.narg(source)::text IS NULL OR l.source = sqlc.narg(source)::text)
  AND (
    sqlc.arg(search)::text = '' OR
    l.full_name ILIKE '%' || sqlc.arg(search)::text || '%' OR
    l.phone ILIKE '%' || sqlc.arg(search)::text || '%' OR
    l.email ILIKE '%' || sqlc.arg(search)::text || '%'
  )
ORDER BY
  CASE WHEN l.stage NOT IN ('CLOSING','CANCELLED') AND l.next_follow_up_at < NOW() THEN 0 ELSE 1 END,
  l.updated_at DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: CreateCRMLeadActivity :one
INSERT INTO crm_lead_activities (
  lead_id, operator_id, branch_id, kind, from_stage, to_stage, note,
  actor_user_id, idempotency_key, request_fingerprint, occurred_at
) VALUES (
  sqlc.arg(lead_id), sqlc.arg(operator_id), sqlc.narg(branch_id)::uuid,
  sqlc.arg(kind), sqlc.narg(from_stage)::text, sqlc.narg(to_stage)::text,
  sqlc.arg(note), sqlc.arg(actor_user_id), sqlc.arg(idempotency_key),
  sqlc.arg(request_fingerprint), sqlc.arg(occurred_at)
)
RETURNING *;

-- name: GetCRMActivityByIdempotency :one
SELECT * FROM crm_lead_activities
WHERE operator_id = sqlc.arg(operator_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid);

-- name: SetCRMLeadInitialActivity :exec
UPDATE crm_leads
SET last_activity_id = sqlc.arg(activity_id)
WHERE id = sqlc.arg(id) AND operator_id = sqlc.arg(operator_id)
  AND last_activity_id IS NULL
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid);

-- name: UpdateCRMLeadProfile :one
UPDATE crm_leads
SET full_name = sqlc.arg(full_name), phone = sqlc.arg(phone), email = sqlc.arg(email),
    source = sqlc.arg(source), campaign = sqlc.arg(campaign),
    season_id = sqlc.narg(season_id)::uuid, product_id = sqlc.narg(product_id)::uuid,
    assignee_user_id = sqlc.narg(assignee_user_id)::text, pax = sqlc.arg(pax),
    estimated_value_idr = sqlc.arg(estimated_value_idr),
    next_action = sqlc.arg(next_action),
    next_follow_up_at = sqlc.narg(next_follow_up_at)::timestamptz,
    last_activity_id = sqlc.arg(activity_id)
WHERE id = sqlc.arg(id) AND operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING *;

-- name: MoveCRMLeadStage :one
UPDATE crm_leads
SET stage = sqlc.arg(stage),
    closed_at = CASE WHEN sqlc.arg(stage)::text = 'CLOSING' THEN NOW() ELSE NULL END,
    last_activity_id = sqlc.arg(activity_id)
WHERE id = sqlc.arg(id) AND operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING *;

-- name: ApplyCRMLeadActivity :one
UPDATE crm_leads
SET last_contact_at = CASE WHEN sqlc.arg(kind)::text IN ('CONTACT','OFFER_SENT')
                           THEN sqlc.arg(occurred_at) ELSE last_contact_at END,
    next_action = sqlc.arg(next_action),
    next_follow_up_at = sqlc.narg(next_follow_up_at)::timestamptz,
    last_activity_id = sqlc.arg(activity_id)
WHERE id = sqlc.arg(id) AND operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING *;

-- name: ListCRMLeadActivities :many
SELECT a.*, COALESCE(u.name, '') AS actor_name
FROM crm_lead_activities a
LEFT JOIN "user" u ON u.id = a.actor_user_id
WHERE a.lead_id = sqlc.arg(lead_id) AND a.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR a.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY a.occurred_at DESC, a.created_at DESC;

-- name: GetCRMPipelineSummary :one
SELECT
  COUNT(*) FILTER (WHERE stage NOT IN ('CLOSING','CANCELLED'))::bigint AS active_count,
  COALESCE(SUM(estimated_value_idr) FILTER (WHERE stage NOT IN ('CLOSING','CANCELLED')), 0)::bigint AS pipeline_value_idr,
  COUNT(*) FILTER (
    WHERE stage NOT IN ('CLOSING','CANCELLED') AND next_follow_up_at < NOW()
  )::bigint AS overdue_follow_up_count,
  COUNT(DISTINCT source) FILTER (WHERE stage NOT IN ('CLOSING','CANCELLED'))::bigint AS source_count,
  COUNT(*) FILTER (WHERE stage = 'CLOSING' AND closed_at >= date_trunc('month', NOW()))::bigint AS monthly_closing_count,
  COUNT(*) FILTER (WHERE created_at >= date_trunc('month', NOW()))::bigint AS monthly_created_count
FROM crm_leads
WHERE operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid);

-- name: ListCRMStageMetrics :many
SELECT stage, COUNT(*)::bigint AS lead_count,
       COALESCE(SUM(estimated_value_idr), 0)::bigint AS value_idr
FROM crm_leads
WHERE operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid)
GROUP BY stage ORDER BY stage;

-- name: ListCRMSourceMetrics :many
SELECT source, COUNT(*)::bigint AS lead_count,
       COALESCE(SUM(estimated_value_idr), 0)::bigint AS value_idr
FROM crm_leads
WHERE operator_id = sqlc.arg(operator_id)
  AND stage NOT IN ('CANCELLED')
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid)
GROUP BY source ORDER BY lead_count DESC, source;

-- name: ListCRMAssigneeMetrics :many
SELECT COALESCE(l.assignee_user_id, '') AS user_id,
       COALESCE(u.name, 'Belum ditugaskan') AS name,
       COUNT(*) FILTER (WHERE l.stage NOT IN ('CLOSING','CANCELLED'))::bigint AS active_count,
       COUNT(*) FILTER (WHERE l.stage = 'CLOSING')::bigint AS closing_count,
       COALESCE(SUM(l.estimated_value_idr) FILTER (WHERE l.stage = 'CLOSING'), 0)::bigint AS value_idr
FROM crm_leads l
LEFT JOIN "user" u ON u.id = l.assignee_user_id
WHERE l.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR l.branch_id = sqlc.narg(branch_scope)::uuid)
GROUP BY l.assignee_user_id, u.name
ORDER BY closing_count DESC, active_count DESC, name
LIMIT 10;

-- name: ListCRMAttentionLeads :many
SELECT l.*, COALESCE(s.name, '') AS season_name,
       COALESCE(p.name, '') AS product_name,
       COALESCE(u.name, '') AS assignee_name
FROM crm_leads l
LEFT JOIN seasons s ON s.id = l.season_id
LEFT JOIN products p ON p.id = l.product_id
LEFT JOIN "user" u ON u.id = l.assignee_user_id
WHERE l.operator_id = sqlc.arg(operator_id)
  AND l.stage NOT IN ('CLOSING','CANCELLED')
  AND l.next_follow_up_at < NOW()
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR l.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY l.next_follow_up_at ASC
LIMIT 8;

-- name: ListCRMAssignees :many
SELECT u.id AS user_id, u.name, u.email
FROM operators o
JOIN "member" m ON m."organizationId" = o.better_auth_org_id
JOIN "user" u ON u.id = m."userId"
WHERE o.id = sqlc.arg(operator_id)
  AND (
    sqlc.narg(branch_scope)::uuid IS NULL OR
    u.id IN (
      SELECT bm.better_auth_user_id FROM branch_members bm
      WHERE bm.operator_id = o.id AND bm.branch_id = sqlc.narg(branch_scope)::uuid
    )
  )
ORDER BY u.name, u.email;
