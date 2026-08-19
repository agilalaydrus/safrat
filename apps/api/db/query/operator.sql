-- name: CreateOperator :one
INSERT INTO operators (better_auth_org_id, name, country, email, license_number)
VALUES ($1, $2, $3, $4, NULLIF($5, ''))
ON CONFLICT (better_auth_org_id) DO NOTHING
RETURNING *;

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
ORDER BY a.created_at DESC
LIMIT $2;
