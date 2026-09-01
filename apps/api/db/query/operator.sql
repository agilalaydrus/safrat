-- name: CreateOperator :one
INSERT INTO operators (better_auth_org_id, name, country, email, license_number, slug)
VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
ON CONFLICT (better_auth_org_id) DO NOTHING
RETURNING *;

-- name: OperatorSlugExists :one
SELECT EXISTS(SELECT 1 FROM operators WHERE slug = $1);

-- name: GetOperatorBySlug :one
SELECT * FROM operators WHERE slug = $1;

-- name: UpdateOperator :one
UPDATE operators
SET name = $2, country = $3, email = $4, license_number = NULLIF($5, '')
WHERE id = $1
RETURNING *;

-- name: GetOperatorByBetterAuthOrgID :one
SELECT * FROM operators WHERE better_auth_org_id = $1;

-- name: GetOperatorByID :one
SELECT * FROM operators WHERE id = $1;

-- name: ListOperatorIDs :many
SELECT id FROM operators;

-- name: ListAuditLogs :many
SELECT a.id, a.operator_id, a.action, a.entity_type, a.entity_id,
       COALESCE(a.metadata ->> 'message', a.action) AS description,
       a.created_at,
       COALESCE(u.name, '') AS actor_name
FROM audit_logs a
LEFT JOIN "user" u ON u.id = a.user_id
WHERE a.operator_id = $1
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR a.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY a.created_at DESC
LIMIT $2;

-- name: UpdateOperatorProfile :one
UPDATE operators SET
  logo_url            = $2,
  description         = $3,
  whatsapp_number     = $4,
  website             = $5,
  address             = $6,
  city                = $7,
  brand_color         = $8,
  hero_eyebrow        = $9,
  hero_title          = $10,
  hero_subtitle       = $11,
  hero_image_url      = $12,
  is_profile_complete = TRUE
WHERE id = $1
RETURNING *;
