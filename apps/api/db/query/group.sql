-- name: ListGroupsByLeader :many
SELECT g.*, COUNT(p.id)::int AS pilgrim_count
FROM groups g
LEFT JOIN pilgrims p ON p.group_id = g.id AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
WHERE g.operator_id = $1 AND g.leader_id = $2
GROUP BY g.id
ORDER BY g.name ASC;

-- name: GetGroupForOperator :one
SELECT * FROM groups
WHERE id = $1 AND operator_id = $2;

-- name: GetGroupForLeader :one
SELECT * FROM groups
WHERE id = $1 AND operator_id = $2 AND leader_id = $3;

-- name: ListGroupRoster :many
SELECT * FROM pilgrims
WHERE group_id = $1 AND operator_id = $2 AND is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY full_name ASC;

-- name: ListGroupsForOperator :many
SELECT g.*, COUNT(p.id)::int AS pilgrim_count, u.name AS leader_name
FROM groups g
LEFT JOIN pilgrims p ON p.group_id = g.id AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
LEFT JOIN "user" u ON u.id = g.leader_id
WHERE g.operator_id = $1 AND g.season_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR EXISTS (SELECT 1 FROM pilgrims gp WHERE gp.group_id = g.id AND gp.branch_id = sqlc.narg(branch_scope)::uuid))
GROUP BY g.id, u.name
ORDER BY g.name ASC;

-- name: ListGroupsByKloter :many
SELECT g.*, COUNT(p.id)::int AS pilgrim_count, u.name AS leader_name
FROM groups g
LEFT JOIN pilgrims p ON p.group_id = g.id AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
LEFT JOIN "user" u ON u.id = g.leader_id
WHERE g.operator_id = $1 AND g.kloter_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR EXISTS (SELECT 1 FROM pilgrims gp WHERE gp.group_id = g.id AND gp.branch_id = sqlc.narg(branch_scope)::uuid))
GROUP BY g.id, u.name
ORDER BY g.name ASC;

-- name: UpdateGroupCity :one
UPDATE groups
SET current_city = $3, current_activity = $4, last_update = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: InsertGroupLocationLog :exec
INSERT INTO group_location_log (operator_id, group_id, city, location, updated_by)
VALUES ($1, $2, $3, $4, $5);

-- name: CreateGroup :one
INSERT INTO groups (operator_id, season_id, name, capacity)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateGroup :one
UPDATE groups
SET name = $3, capacity = $4, leader_id = NULLIF($5::text, ''), kloter_id = NULLIF($6::text, '')::uuid
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: UnassignGroupPilgrims :exec
UPDATE pilgrims SET group_id = NULL WHERE group_id = $1 AND operator_id = $2;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = $1 AND operator_id = $2;

-- name: ListMuttawwif :many
-- One row per (leader, group) pair, operator-wide (not season-scoped,
-- unlike ListGroupsForOperator) — the service layer aggregates rows sharing
-- a user_id into one Muttawwif with multiple groups.
SELECT g.id AS group_id, g.name AS group_name, g.capacity, g.season_id,
       COUNT(p.id)::int AS pilgrim_count,
       u.id AS user_id, u.name AS user_name, u.email AS user_email,
       COALESCE(a.phone, '') AS phone,
       a.id AS agent_id,
       COALESCE(a.kyc_status, 'UNVERIFIED') AS kyc_status
FROM groups g
JOIN "user" u ON u.id = g.leader_id
LEFT JOIN pilgrims p ON p.group_id = g.id AND p.is_substituted = false
LEFT JOIN agents a ON a.linked_user_id = g.leader_id AND a.operator_id = g.operator_id
WHERE g.operator_id = $1
GROUP BY g.id, u.id, u.name, u.email, a.phone, a.id, a.kyc_status
ORDER BY u.name ASC, g.name ASC;

-- name: ListOperatorMembers :many
SELECT u.id, u.name, u.email
FROM member m
JOIN "user" u ON u.id = m."userId"
WHERE m."organizationId" = $1
ORDER BY u.name ASC;

-- name: PilgrimBelongsToLeader :one
-- Ownership check for leader-scoped pilgrim actions (hotel check-in,
-- movement check-in) — confirms the pilgrim is in a group this leader
-- actually leads, not just any pilgrim in the operator.
SELECT p.id FROM pilgrims p
JOIN groups g ON g.id = p.group_id
WHERE p.id = $1 AND p.operator_id = $2 AND g.leader_id = $3;

-- name: LeaderHasPilgrimInKloter :one
-- Confirms this leader has at least one of their own pilgrims in the given
-- kloter — used to scope ListCheckIns (which only has a movement_id, not a
-- pilgrim_id) to movements actually relevant to this leader's group.
SELECT 1 FROM pilgrims p
JOIN groups g ON g.id = p.group_id
WHERE g.operator_id = $1 AND g.leader_id = $2 AND p.kloter_id = $3
LIMIT 1;
