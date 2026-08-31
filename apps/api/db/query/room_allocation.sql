-- name: AllocatePilgrimTx :one
INSERT INTO room_allocations (room_id, hotel_id, pilgrim_id, operator_id, assigned_by)
SELECT sqlc.arg(room_id), sqlc.arg(hotel_id), p.id, sqlc.arg(operator_id), sqlc.arg(assigned_by)
FROM pilgrims p
JOIN rooms r ON r.id = sqlc.arg(room_id)
  AND r.hotel_id = sqlc.arg(hotel_id)
  AND r.operator_id = sqlc.arg(operator_id)
WHERE p.id = sqlc.arg(pilgrim_id)
  AND p.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING *;
-- name: DeallocatePilgrim :execrows
-- Scoped to the specific room, not just the pilgrim — a pilgrim can now
-- hold one allocation per hotel (Makkah + Madinah), so removing one must
-- never wipe the other.
DELETE FROM room_allocations ra USING pilgrims p
WHERE ra.pilgrim_id = sqlc.arg(pilgrim_id)
  AND ra.operator_id = sqlc.arg(operator_id)
  AND ra.room_id = sqlc.arg(room_id)
  AND p.id = ra.pilgrim_id
  AND p.operator_id = ra.operator_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: TransferAllocation :execrows
-- Substitution cascade — moves every allocation the original pilgrim held
-- (across every hotel) to the replacement, in one shot.
UPDATE room_allocations
SET pilgrim_id = sqlc.arg(replacement_id)
FROM pilgrims original, pilgrims replacement
WHERE room_allocations.pilgrim_id = sqlc.arg(original_id)
  AND room_allocations.operator_id = sqlc.arg(operator_id)
  AND original.id = room_allocations.pilgrim_id
  AND original.operator_id = room_allocations.operator_id
  AND replacement.id = sqlc.arg(replacement_id)
  AND replacement.operator_id = room_allocations.operator_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR (
    original.branch_id = sqlc.narg(branch_scope)::uuid
    AND replacement.branch_id = sqlc.narg(branch_scope)::uuid
  ));
-- name: GetAllocationForHotel :one
-- Used by AllocatePilgrim to block a second room in the SAME hotel, not a
-- second room overall (that's the whole point — Makkah + Madinah is valid).
SELECT ra.* FROM room_allocations ra
JOIN pilgrims p ON p.id = ra.pilgrim_id AND p.operator_id = ra.operator_id
WHERE ra.pilgrim_id = sqlc.arg(pilgrim_id)
  AND ra.hotel_id = sqlc.arg(hotel_id)
  AND ra.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);
-- name: ListAllocationsByPilgrim :many
SELECT ra.* FROM room_allocations ra
JOIN pilgrims p ON p.id = ra.pilgrim_id AND p.operator_id = ra.operator_id
WHERE ra.pilgrim_id = sqlc.arg(pilgrim_id)
  AND ra.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);
-- name: ListAllocationsByRoom :many
SELECT ra.* FROM room_allocations ra
JOIN pilgrims p ON p.id = ra.pilgrim_id AND p.operator_id = ra.operator_id
WHERE ra.room_id = sqlc.arg(room_id)
  AND ra.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY ra.allocated_at;
-- name: CountAllocatedByRoom :one
-- Capacity is a shared physical constraint. Keep this operator-wide even for
-- branch heads, otherwise two branches could overbook the same room.
SELECT COUNT(*) FROM room_allocations WHERE room_id = $1 AND operator_id = $2;

-- name: ListPilgrimRoomAssignments :many
-- One row per (pilgrim, hotel) — a pilgrim legitimately has more than one
-- (Makkah + Madinah), so this is no longer collapsed to a single row.
SELECT
  p.id AS pilgrim_id, ra.room_id, ra.hotel_id, h.name AS hotel_name, r.room_number, r.room_type
FROM pilgrims p
JOIN room_allocations ra ON ra.pilgrim_id = p.id
JOIN rooms r ON r.id = ra.room_id
JOIN hotels h ON h.id = r.hotel_id
WHERE p.operator_id = sqlc.arg(operator_id)
  AND p.season_id = sqlc.arg(season_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY p.id, ra.allocated_at DESC;
