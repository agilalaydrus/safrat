-- name: AllocatePilgrimTx :one
INSERT INTO room_allocations (room_id, pilgrim_id, operator_id, assigned_by)
VALUES ($1, $2, $3, $4) RETURNING *;
-- name: DeallocatePilgrim :exec
DELETE FROM room_allocations WHERE pilgrim_id = $1 AND operator_id = $2;

-- name: TransferAllocation :execrows
UPDATE room_allocations
SET pilgrim_id = $2
WHERE pilgrim_id = $1 AND operator_id = $3;
-- name: GetAllocationByPilgrim :one
SELECT * FROM room_allocations WHERE pilgrim_id = $1 AND operator_id = $2;
-- name: ListAllocationsByRoom :many
SELECT * FROM room_allocations WHERE room_id = $1 AND operator_id = $2 ORDER BY allocated_at;
-- name: CountAllocatedByRoom :one
SELECT COUNT(*) FROM room_allocations WHERE room_id = $1;

-- name: ListPilgrimRoomAssignments :many
-- A pilgrim can hold one room allocation per hotel in a season (e.g. Makkah +
-- Madinah); DISTINCT ON picks the most recently allocated as the one shown —
-- same simplification as GetPilgrimAppInfo's Home card.
SELECT DISTINCT ON (p.id)
  p.id AS pilgrim_id, h.name AS hotel_name, r.room_number, r.room_type
FROM pilgrims p
JOIN room_allocations ra ON ra.pilgrim_id = p.id
JOIN rooms r ON r.id = ra.room_id
JOIN hotels h ON h.id = r.hotel_id
WHERE p.operator_id = $1 AND p.season_id = $2
ORDER BY p.id, ra.allocated_at DESC;
