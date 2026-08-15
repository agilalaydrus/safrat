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

-- name: ListGroupsForOperator :many
SELECT g.*, COUNT(p.id)::int AS pilgrim_count, u.name AS leader_name
FROM groups g
LEFT JOIN pilgrims p ON p.group_id = g.id AND p.is_substituted = false
LEFT JOIN "user" u ON u.id = g.leader_id
WHERE g.operator_id = $1 AND g.season_id = $2
GROUP BY g.id, u.name
ORDER BY g.name ASC;

-- name: CreateGroup :one
INSERT INTO groups (operator_id, season_id, name, capacity)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateGroup :one
UPDATE groups
SET name = $3, capacity = $4, leader_id = NULLIF($5::text, '')
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: UnassignGroupPilgrims :exec
UPDATE pilgrims SET group_id = NULL WHERE group_id = $1 AND operator_id = $2;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = $1 AND operator_id = $2;

-- name: ListOperatorMembers :many
SELECT u.id, u.name, u.email
FROM member m
JOIN "user" u ON u.id = m."userId"
WHERE m."organizationId" = $1
ORDER BY u.name ASC;
