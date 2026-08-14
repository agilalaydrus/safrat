-- name: CreateVehicle :one
INSERT INTO vehicles (movement_id,operator_id,plate_number,capacity,driver_name,driver_phone) VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')) RETURNING *;
-- name: GetVehicle :one
SELECT * FROM vehicles WHERE id=$1 AND operator_id=$2;
-- name: ListVehiclesByMovement :many
SELECT v.*, COUNT(sa.id)::int AS assigned_count FROM vehicles v LEFT JOIN seat_assignments sa ON sa.vehicle_id=v.id WHERE v.movement_id=$1 AND v.operator_id=$2 GROUP BY v.id ORDER BY v.plate_number ASC;
-- name: LockVehicle :one
SELECT id, capacity FROM vehicles WHERE id=$1 FOR UPDATE;
-- name: CountAssignedByVehicle :one
SELECT COUNT(*)::int FROM seat_assignments WHERE vehicle_id=$1;
-- name: UpdateVehicleStatus :one
UPDATE vehicles SET status=$3, departed_at=CASE WHEN $3='departed' THEN NOW() ELSE departed_at END, arrived_at=CASE WHEN $3='arrived' THEN NOW() ELSE arrived_at END, updated_at=NOW() WHERE id=$1 AND operator_id=$2 RETURNING *;
-- name: DeleteVehicle :exec
DELETE FROM vehicles WHERE id=$1 AND operator_id=$2;
