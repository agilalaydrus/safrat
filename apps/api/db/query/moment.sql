-- name: CreateMoment :one
INSERT INTO pilgrim_moments (operator_id, season_id, pilgrim_id, group_id, photo_key, caption, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: DeleteMoment :one
DELETE FROM pilgrim_moments WHERE id = $1 AND operator_id = $2 RETURNING photo_key;

-- name: ListMoments :many
-- Staff-facing list for the dashboard: every moment in the season, whoever
-- it targets.
SELECT m.*, COALESCE(p.full_name, '') AS pilgrim_name, COALESCE(g.name, '') AS group_name
FROM pilgrim_moments m
LEFT JOIN pilgrims p ON p.id = m.pilgrim_id
LEFT JOIN groups g ON g.id = m.group_id
WHERE m.operator_id = $1 AND m.season_id = $2
ORDER BY m.created_at DESC;

-- name: ListFamilyMoments :many
-- Family-facing: every moment addressed to this pilgrim directly, or to the
-- group they belong to.
SELECT m.id, m.photo_key, m.caption, m.created_at
FROM pilgrim_moments m
WHERE m.pilgrim_id = $1
   OR (m.group_id IS NOT NULL AND m.group_id = (SELECT group_id FROM pilgrims WHERE id = $1))
ORDER BY m.created_at DESC
LIMIT 30;
