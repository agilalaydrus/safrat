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

-- name: GetAgentByLinkedUser :one
SELECT * FROM agents WHERE operator_id = $1 AND linked_user_id = $2;

-- name: CreateAgentForLeader :one
-- referral_code/tier/commission_rate/notes all take their column DEFAULT —
-- an auto-created leader-agent starts identical to a freshly hand-created
-- one, just pre-filled from their real name/email instead of a form.
INSERT INTO agents (operator_id, name, phone, email, linked_user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserForAgent :one
SELECT id, name, email FROM "user" WHERE id = $1;

-- name: UpdateAgentKyc :one
UPDATE agents
SET nik = $3, npwp = $4, address = $5, date_of_birth = $6, passport_number = $7,
    passport_expiry_date = $8, bank_name = $9, bank_account_number = $10, bank_account_holder = $11,
    kyc_status = $12, kyc_source = $13, kyc_verified_by = '', kyc_verified_at = NULL, kyc_rejection_reason = '',
    updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: VerifyAgentKyc :one
UPDATE agents
SET kyc_status = $3, kyc_verified_by = $4, kyc_verified_at = NOW(), kyc_rejection_reason = $5, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: CreateAgentDocument :one
INSERT INTO agent_documents (agent_id, operator_id, doc_type, file_url, file_name, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAgentDocuments :many
SELECT * FROM agent_documents WHERE agent_id = $1 ORDER BY created_at DESC;

-- name: ListMyPilgrims :many
-- Agent-facing: list pilgrims referred by this agent, across all seasons.
SELECT
  p.id, p.full_name, p.passport_number, p.gender, p.payment_status,
  (p.documents_passport AND p.documents_photo AND p.documents_vaccine) AS docs_complete,
  p.status AS pilgrim_status,
  s.id AS season_id,
  s.name AS season_name,
  s.start_date AS departure_date
FROM pilgrims p
JOIN seasons s ON s.id = p.season_id
WHERE p.agent_id = $1 AND p.operator_id = $2
ORDER BY s.start_date DESC, p.full_name ASC;
