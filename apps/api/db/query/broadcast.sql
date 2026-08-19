-- name: CreateBroadcast :one
INSERT INTO broadcasts (operator_id, season_id, title, body)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListBroadcasts :many
SELECT * FROM broadcasts
WHERE operator_id = $1 AND season_id = $2
ORDER BY created_at DESC;

-- name: DeleteBroadcast :exec
DELETE FROM broadcasts WHERE id = $1 AND operator_id = $2;
