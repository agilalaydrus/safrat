-- name: CreateSOSAlert :one
INSERT INTO sos_alerts (operator_id, pilgrim_id)
VALUES ($1, $2)
RETURNING *;

-- name: ListActiveSOSAlerts :many
SELECT s.*, p.full_name AS pilgrim_name
FROM sos_alerts s
JOIN pilgrims p ON p.id = s.pilgrim_id
WHERE s.operator_id = $1 AND s.status IN ('ACTIVE','ACKNOWLEDGED','ESCALATED')
ORDER BY s.created_at DESC;

-- name: ListActiveSOSAlertsForLeader :many
SELECT s.*, p.full_name AS pilgrim_name
FROM sos_alerts s
JOIN pilgrims p ON p.id = s.pilgrim_id
JOIN groups g ON g.id = p.group_id
WHERE s.operator_id = $1 AND g.leader_id = $2 AND s.status IN ('ACTIVE','ACKNOWLEDGED','ESCALATED')
ORDER BY s.created_at DESC;

-- name: GetSOSAlert :one
SELECT * FROM sos_alerts
WHERE id = $1 AND operator_id = $2;

-- name: AcknowledgeSOSAlert :one
UPDATE sos_alerts
SET status = 'ACKNOWLEDGED', acknowledged_by = $3, acknowledged_at = NOW()
WHERE id = $1 AND operator_id = $2 AND status = 'ACTIVE'
RETURNING *;

-- name: ResolveSOSAlert :one
UPDATE sos_alerts
SET status = 'RESOLVED', resolved_by = $3, resolved_at = NOW(), notes = $4
WHERE id = $1 AND operator_id = $2 AND status != 'RESOLVED'
RETURNING *;

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
