-- name: CreateRoom :one
INSERT INTO rooms (hotel_id, operator_id, room_number, room_type, capacity, floor, notes, gender)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8) RETURNING *;

-- name: BulkCreateRooms :many
INSERT INTO rooms (hotel_id, operator_id, room_number, room_type, capacity, floor, notes, gender)
SELECT $1, $2, room_number, $4, $5, $6, NULLIF($7, ''), $8
FROM unnest($3::text[]) AS room_number
RETURNING *;
-- name: GetRoom :one
SELECT * FROM rooms WHERE id = $1 AND operator_id = $2;
-- name: ListRoomsByHotel :many
SELECT r.*, COUNT(ra.id)::int AS allocated_count FROM rooms r
LEFT JOIN room_allocations ra ON ra.room_id = r.id
WHERE r.hotel_id = $1 AND r.operator_id = $2
GROUP BY r.id ORDER BY r.room_number;
-- name: GetRoomWithAllocations :one
SELECT r.*, COALESCE(array_agg(ra.pilgrim_id) FILTER (WHERE ra.pilgrim_id IS NOT NULL), '{}')::uuid[] AS pilgrim_ids
FROM rooms r LEFT JOIN room_allocations ra ON ra.room_id = r.id
WHERE r.id = $1 AND r.operator_id = $2 GROUP BY r.id;
-- name: DeleteRoom :exec
DELETE FROM rooms WHERE id = $1 AND operator_id = $2;
