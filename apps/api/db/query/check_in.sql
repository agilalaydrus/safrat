-- name: CreateCheckIn :one
INSERT INTO check_ins (operator_id, movement_id, pilgrim_id, type, checked_in_by)
VALUES ($1, $2, $3, $4, NULLIF($5::text, ''))
RETURNING *;

-- name: ListCheckInsByMovement :many
SELECT * FROM check_ins
WHERE movement_id = $1 AND operator_id = $2
ORDER BY created_at ASC;
