-- name: ListGroupsByLeader :many
SELECT g.*, COUNT(p.id)::int AS pilgrim_count
FROM groups g
LEFT JOIN pilgrims p ON p.group_id = g.id
WHERE g.operator_id = $1 AND g.leader_id = $2
GROUP BY g.id
ORDER BY g.name ASC;

-- name: GetGroupForLeader :one
SELECT * FROM groups
WHERE id = $1 AND operator_id = $2 AND leader_id = $3;

-- name: ListGroupRoster :many
SELECT * FROM pilgrims
WHERE group_id = $1 AND operator_id = $2 AND is_substituted = false
ORDER BY full_name ASC;
