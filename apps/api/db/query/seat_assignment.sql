-- name: AssignSeat :one
INSERT INTO seat_assignments (vehicle_id,pilgrim_id,operator_id,seat_number,assigned_by)
SELECT sqlc.arg(vehicle_id), p.id, sqlc.arg(operator_id), sqlc.arg(seat_number), sqlc.arg(assigned_by)
FROM pilgrims p
JOIN vehicles v ON v.id = sqlc.arg(vehicle_id) AND v.operator_id = sqlc.arg(operator_id)
WHERE p.id = sqlc.arg(pilgrim_id) AND p.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ON CONFLICT (vehicle_id,pilgrim_id) DO UPDATE SET seat_number=EXCLUDED.seat_number,assigned_by=EXCLUDED.assigned_by,assigned_at=NOW()
RETURNING *;
-- name: UnassignSeat :execrows
DELETE FROM seat_assignments sa USING pilgrims p
WHERE sa.vehicle_id=$1 AND sa.pilgrim_id=$2 AND sa.operator_id=$3 AND p.id=sa.pilgrim_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);
-- name: UnassignPilgrimAllSeats :execrows
DELETE FROM seat_assignments sa USING pilgrims p
WHERE sa.pilgrim_id=$1 AND sa.operator_id=$2 AND p.id=sa.pilgrim_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);
-- name: GetVehicleManifest :many
SELECT sa.id,sa.vehicle_id,sa.pilgrim_id,sa.seat_number,sa.assigned_at,p.full_name,p.gender,p.passport_number,p.requires_wheelchair
FROM seat_assignments sa JOIN pilgrims p ON p.id=sa.pilgrim_id
WHERE sa.vehicle_id=$1 AND sa.operator_id=$2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY sa.seat_number ASC NULLS LAST,p.full_name ASC;
-- name: IsSeatNumberTaken :one
SELECT EXISTS(SELECT 1 FROM seat_assignments WHERE vehicle_id=$1 AND seat_number=$2 AND pilgrim_id != $3) AS taken;
