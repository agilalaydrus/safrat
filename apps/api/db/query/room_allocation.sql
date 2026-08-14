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
