-- name: AllocatePilgrimTx :one
INSERT INTO room_allocations (room_id, hotel_id, pilgrim_id, operator_id, assigned_by)
VALUES ($1, $2, $3, $4, $5) RETURNING *;
-- name: DeallocatePilgrim :exec
-- Scoped to the specific room, not just the pilgrim — a pilgrim can now
-- hold one allocation per hotel (Makkah + Madinah), so removing one must
-- never wipe the other.
DELETE FROM room_allocations WHERE pilgrim_id = $1 AND operator_id = $2 AND room_id = $3;

-- name: TransferAllocation :execrows
-- Substitution cascade — moves every allocation the original pilgrim held
-- (across every hotel) to the replacement, in one shot.
UPDATE room_allocations
SET pilgrim_id = $2
WHERE pilgrim_id = $1 AND operator_id = $3;
-- name: GetAllocationForHotel :one
-- Used by AllocatePilgrim to block a second room in the SAME hotel, not a
-- second room overall (that's the whole point — Makkah + Madinah is valid).
SELECT * FROM room_allocations WHERE pilgrim_id = $1 AND hotel_id = $2 AND operator_id = $3;
-- name: ListAllocationsByPilgrim :many
SELECT * FROM room_allocations WHERE pilgrim_id = $1 AND operator_id = $2;
-- name: ListAllocationsByRoom :many
SELECT * FROM room_allocations WHERE room_id = $1 AND operator_id = $2 ORDER BY allocated_at;
-- name: CountAllocatedByRoom :one
SELECT COUNT(*) FROM room_allocations WHERE room_id = $1;

-- name: ListPilgrimRoomAssignments :many
-- One row per (pilgrim, hotel) — a pilgrim legitimately has more than one
-- (Makkah + Madinah), so this is no longer collapsed to a single row.
SELECT
  p.id AS pilgrim_id, ra.room_id, ra.hotel_id, h.name AS hotel_name, r.room_number, r.room_type
FROM pilgrims p
JOIN room_allocations ra ON ra.pilgrim_id = p.id
JOIN rooms r ON r.id = ra.room_id
JOIN hotels h ON h.id = r.hotel_id
WHERE p.operator_id = $1 AND p.season_id = $2
ORDER BY p.id, ra.allocated_at DESC;
