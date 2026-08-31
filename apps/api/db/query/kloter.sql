-- name: ListKloters :many
SELECT k.*, COUNT(p.id)::int AS pilgrim_count
FROM kloters k
LEFT JOIN pilgrims p ON p.kloter_id = k.id AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
WHERE k.operator_id = $1 AND k.season_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR EXISTS (SELECT 1 FROM pilgrims gp WHERE gp.kloter_id = k.id AND gp.branch_id = sqlc.narg(branch_scope)::uuid))
GROUP BY k.id
ORDER BY k.departure_date ASC NULLS LAST, k.code ASC;

-- name: GetKloterForOperator :one
SELECT * FROM kloters
WHERE id = $1 AND operator_id = $2;

-- name: GetKloterForOperatorForUpdate :one
SELECT * FROM kloters
WHERE id = $1 AND operator_id = $2
FOR UPDATE;

-- name: CreateKloter :one
INSERT INTO kloters (operator_id, season_id, code, embarkation, flight_number, departure_date, capacity)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateKloter :one
UPDATE kloters
SET code = $3, embarkation = $4, flight_number = $5, departure_date = $6, capacity = $7
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: UpdateKloterStatus :one
UPDATE kloters
SET status = $3
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: UnassignKloterPilgrims :exec
UPDATE pilgrims SET kloter_id = NULL WHERE kloter_id = $1 AND operator_id = $2;

-- name: DeleteKloter :exec
DELETE FROM kloters WHERE id = $1 AND operator_id = $2;
