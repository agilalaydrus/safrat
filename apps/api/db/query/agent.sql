-- name: CreateAgent :one
INSERT INTO agents (operator_id, name, phone, email, commission_rate, notes, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents
WHERE id = $1 AND operator_id = $2;

-- name: ListAgentsWithPilgrimCount :many
SELECT a.*, COUNT(p.id)::int AS pilgrim_count
FROM agents a
LEFT JOIN pilgrims p ON p.agent_id = a.id
WHERE a.operator_id = $1
GROUP BY a.id
ORDER BY a.name ASC;

-- name: UpdateAgent :one
UPDATE agents
SET name = $3, phone = $4, email = $5, commission_rate = $6,
    notes = $7, is_active = $8, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteAgent :exec
DELETE FROM agents
WHERE id = $1 AND operator_id = $2;

-- name: AssignPilgrimToAgent :exec
UPDATE pilgrims
SET agent_id = $2, updated_at = NOW()
WHERE id = $1 AND operator_id = $3;

-- name: CreateAgentApplication :one
INSERT INTO agents (operator_id, name, phone, email, is_active, referred_by_agent_id)
VALUES ($1, $2, $3, $4, false, NULLIF($5::text, '')::uuid)
RETURNING *;

-- name: GetAgentByReferralCode :one
SELECT * FROM agents
WHERE referral_code = $1 AND operator_id = $2;

-- name: UpdateAgentTier :exec
UPDATE agents
SET tier = $3, updated_at = NOW()
WHERE id = $1 AND operator_id = $2;

-- name: ListActiveAgentsForTiering :many
SELECT a.id, a.operator_id, a.tier, COUNT(p.id)::int AS pilgrim_count
FROM agents a
LEFT JOIN pilgrims p ON p.agent_id = a.id
WHERE a.operator_id = $1 AND a.is_active = true
GROUP BY a.id;
