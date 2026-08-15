-- name: CreateOperator :one
INSERT INTO operators (better_auth_org_id, name, country, email, license_number)
VALUES ($1, $2, $3, $4, NULLIF($5, ''))
ON CONFLICT (better_auth_org_id) DO NOTHING
RETURNING *;

-- name: GetOperatorByBetterAuthOrgID :one
SELECT * FROM operators WHERE better_auth_org_id = $1;

-- name: GetOperatorByID :one
SELECT * FROM operators WHERE id = $1;

-- name: ListOperatorIDs :many
SELECT id FROM operators;

-- name: ListAuditLogs :many
SELECT id, operator_id, action, entity_id, COALESCE(metadata ->> 'message', action) AS description, created_at
FROM audit_logs
WHERE operator_id = $1
ORDER BY created_at DESC
LIMIT $2;
