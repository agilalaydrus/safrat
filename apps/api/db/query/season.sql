-- name: CreateSeason :one
INSERT INTO seasons (operator_id, name, type, start_date, end_date)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListSeasonsByOperatorID :many
SELECT * FROM seasons
WHERE operator_id = $1
ORDER BY start_date DESC;

-- name: SetActiveSeason :many
UPDATE seasons
SET is_active = (seasons.id = $1)
WHERE seasons.operator_id = $2
  AND EXISTS (
    SELECT 1 FROM seasons AS target
    WHERE target.id = $1 AND target.operator_id = $2
  )
RETURNING *;
