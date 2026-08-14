-- name: AssignSeat :one
INSERT INTO seat_assignments (vehicle_id,pilgrim_id,operator_id,seat_number,assigned_by) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (vehicle_id,pilgrim_id) DO UPDATE SET seat_number=EXCLUDED.seat_number,assigned_by=EXCLUDED.assigned_by,assigned_at=NOW() RETURNING *;
-- name: UnassignSeat :exec
DELETE FROM seat_assignments WHERE vehicle_id=$1 AND pilgrim_id=$2 AND operator_id=$3;
-- name: UnassignPilgrimAllSeats :execrows
DELETE FROM seat_assignments WHERE pilgrim_id=$1 AND operator_id=$2;
-- name: GetVehicleManifest :many
SELECT sa.id,sa.vehicle_id,sa.pilgrim_id,sa.seat_number,sa.assigned_at,p.full_name,p.gender,p.passport_number,p.requires_wheelchair FROM seat_assignments sa JOIN pilgrims p ON p.id=sa.pilgrim_id WHERE sa.vehicle_id=$1 ORDER BY sa.seat_number ASC NULLS LAST,p.full_name ASC;
-- name: IsSeatNumberTaken :one
SELECT EXISTS(SELECT 1 FROM seat_assignments WHERE vehicle_id=$1 AND seat_number=$2 AND pilgrim_id != $3) AS taken;
