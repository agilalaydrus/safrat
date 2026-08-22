-- name: CreateLostReport :one
INSERT INTO lost_reports (pilgrim_id, operator_id, group_id, latitude, longitude, last_known_location)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ResolveLostReport :exec
UPDATE lost_reports
SET status = 'FOUND', resolved_at = NOW()
WHERE id = $1 AND operator_id = $2;

-- name: ResolveGroupLostReport :execrows
UPDATE lost_reports
SET status = 'FOUND', resolved_at = NOW()
WHERE id = $1 AND group_id = $2;

-- name: ListActiveLostReports :many
SELECT lr.*, p.full_name, p.phone, g.name AS group_name
FROM lost_reports lr
JOIN pilgrims p ON p.id = lr.pilgrim_id
LEFT JOIN groups g ON g.id = lr.group_id
WHERE lr.operator_id = $1 AND lr.status = 'LOST'
ORDER BY lr.created_at DESC;

-- name: ListGroupLostReports :many
SELECT lr.*, p.full_name, p.phone
FROM lost_reports lr
JOIN pilgrims p ON p.id = lr.pilgrim_id
WHERE lr.group_id = $1 AND lr.status = 'LOST'
ORDER BY lr.created_at DESC;
