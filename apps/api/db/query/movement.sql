-- name: CreateMovement :one
INSERT INTO movements (season_id, operator_id, name, origin, destination, scheduled_at, mode, kloter_id) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8::text,'')::uuid) RETURNING *;
-- name: GetMovement :one
SELECT * FROM movements WHERE id=$1 AND operator_id=$2;
-- name: ListMovementsWithStats :many
SELECT m.*, COUNT(DISTINCT v.id)::int AS vehicle_count, COALESCE(SUM(v.capacity),0)::int AS total_capacity, COUNT(DISTINCT sa.id)::int AS assigned_count FROM movements m LEFT JOIN vehicles v ON v.movement_id=m.id LEFT JOIN seat_assignments sa ON sa.vehicle_id=v.id WHERE m.operator_id=$1 AND m.season_id=$2 GROUP BY m.id ORDER BY m.scheduled_at ASC;
-- name: UpdateMovementStatus :one
UPDATE movements SET status=$3, updated_at=NOW() WHERE id=$1 AND operator_id=$2 RETURNING *;
-- name: DeleteMovement :exec
DELETE FROM movements WHERE id=$1 AND operator_id=$2;

-- name: ListMovementsForKloter :many
SELECT m.*, COUNT(DISTINCT v.id)::int AS vehicle_count, COALESCE(SUM(v.capacity),0)::int AS total_capacity, COUNT(DISTINCT sa.id)::int AS assigned_count
FROM movements m
LEFT JOIN vehicles v ON v.movement_id=m.id
LEFT JOIN seat_assignments sa ON sa.vehicle_id=v.id
WHERE m.operator_id=$1 AND m.kloter_id=$2
GROUP BY m.id ORDER BY m.scheduled_at ASC;
