-- name: CreateSOSAlert :one
-- ON CONFLICT DO NOTHING returns no row on a duplicate idempotency_key (the
-- repo detects ErrNoRows and fetches the existing alert). An empty key never
-- conflicts — the partial unique index excludes it — so keyless callers always
-- insert.
INSERT INTO sos_alerts (operator_id, pilgrim_id, lat, lng, idempotency_key)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (pilgrim_id, idempotency_key) WHERE idempotency_key <> '' DO NOTHING
RETURNING *;

-- name: GetSOSAlertByIdempotencyKey :one
SELECT * FROM sos_alerts WHERE pilgrim_id = $1 AND idempotency_key = $2;

-- name: ListActiveSOSAlerts :many
SELECT s.*, p.full_name AS pilgrim_name
FROM sos_alerts s
JOIN pilgrims p ON p.id = s.pilgrim_id
WHERE s.operator_id = $1 AND s.status IN ('ACTIVE','ACKNOWLEDGED','ESCALATED')
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY s.created_at DESC;

-- name: ListActiveSOSAlertsForLeader :many
SELECT s.*, p.full_name AS pilgrim_name
FROM sos_alerts s
JOIN pilgrims p ON p.id = s.pilgrim_id
JOIN groups g ON g.id = p.group_id
WHERE s.operator_id = $1 AND g.leader_id = $2 AND s.status IN ('ACTIVE','ACKNOWLEDGED','ESCALATED')
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY s.created_at DESC;

-- name: GetSOSAlert :one
SELECT s.*
FROM sos_alerts s
JOIN pilgrims p ON p.id = s.pilgrim_id
WHERE s.id = $1 AND s.operator_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: AcknowledgeSOSAlert :one
UPDATE sos_alerts s
SET status = 'ACKNOWLEDGED', acknowledged_by = $3, acknowledged_at = NOW()
FROM pilgrims p
WHERE s.id = $1 AND s.operator_id = $2 AND s.status = 'ACTIVE' AND p.id = s.pilgrim_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING s.*;

-- name: ResolveSOSAlert :one
UPDATE sos_alerts s
SET status = 'RESOLVED', resolved_by = $3, resolved_at = NOW(), notes = $4
FROM pilgrims p
WHERE s.id = $1 AND s.operator_id = $2 AND s.status != 'RESOLVED' AND p.id = s.pilgrim_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING s.*;

-- name: EscalateStaleSOSAlerts :many
WITH escalated AS (
  UPDATE sos_alerts
  SET status = 'ESCALATED'
  WHERE status = 'ACTIVE' AND created_at < NOW() - INTERVAL '10 minutes'
  RETURNING *
)
SELECT escalated.*, p.full_name AS pilgrim_name
FROM escalated
JOIN pilgrims p ON p.id = escalated.pilgrim_id;

-- name: ListSOSPilgrimHistory :many
SELECT * FROM sos_alerts
WHERE pilgrim_id = $1 AND operator_id = $2
ORDER BY created_at DESC
LIMIT 20;

-- name: ListActiveSOSAlertsForKloter :many
SELECT s.*, p.full_name AS pilgrim_name
FROM sos_alerts s
JOIN pilgrims p ON p.id = s.pilgrim_id
WHERE s.operator_id = $1 AND p.kloter_id = $2 AND s.status IN ('ACTIVE','ACKNOWLEDGED','ESCALATED')
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY s.created_at DESC;
