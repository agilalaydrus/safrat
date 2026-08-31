-- name: CreateLostReport :one
INSERT INTO lost_reports (pilgrim_id, operator_id, group_id, latitude, longitude, last_known_location)
SELECT p.id, sqlc.arg(operator_id), g.id, sqlc.arg(latitude), sqlc.arg(longitude), sqlc.arg(last_known_location)
FROM pilgrims p
LEFT JOIN groups g ON g.id = sqlc.arg(group_id) AND g.operator_id = sqlc.arg(operator_id)
WHERE p.id = sqlc.arg(pilgrim_id) AND p.operator_id = sqlc.arg(operator_id)
  AND (sqlc.arg(group_id)::uuid IS NULL OR p.group_id = g.id)
RETURNING *;

-- name: ResolveLostReport :execrows
UPDATE lost_reports lr
SET status = 'FOUND', resolved_at = NOW()
FROM pilgrims p
WHERE lr.id = $1 AND lr.operator_id = $2 AND p.id = lr.pilgrim_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: ResolveGroupLostReport :execrows
UPDATE lost_reports lr
SET status = 'FOUND', resolved_at = NOW()
FROM pilgrims p
WHERE lr.id = $1 AND lr.group_id = $2 AND p.id = lr.pilgrim_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: ListActiveLostReports :many
SELECT lr.*, p.full_name, p.phone, g.name AS group_name
FROM lost_reports lr
JOIN pilgrims p ON p.id = lr.pilgrim_id
LEFT JOIN groups g ON g.id = lr.group_id
WHERE lr.operator_id = $1 AND lr.status = 'LOST'
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY lr.created_at DESC;

-- name: ListGroupLostReports :many
SELECT lr.*, p.full_name, p.phone
FROM lost_reports lr
JOIN pilgrims p ON p.id = lr.pilgrim_id
WHERE lr.operator_id = $1 AND lr.group_id = $2 AND lr.status = 'LOST'
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY lr.created_at DESC;
