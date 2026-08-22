-- name: CreateHealthReport :one
INSERT INTO pilgrim_health_reports (operator_id, pilgrim_id, group_id, reported_by, severity, symptoms, action_taken)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListHealthReports :many
SELECT hr.*, p.full_name AS pilgrim_name, g.name AS group_name
FROM pilgrim_health_reports hr
JOIN pilgrims p ON p.id = hr.pilgrim_id
JOIN groups g ON g.id = hr.group_id
WHERE hr.operator_id = $1 AND (sqlc.narg('resolved')::boolean IS NULL OR hr.resolved = sqlc.narg('resolved'))
ORDER BY hr.created_at DESC;

-- name: ListHealthReportsForGroup :many
SELECT hr.*, p.full_name AS pilgrim_name, g.name AS group_name
FROM pilgrim_health_reports hr
JOIN pilgrims p ON p.id = hr.pilgrim_id
JOIN groups g ON g.id = hr.group_id
WHERE hr.operator_id = $1 AND hr.group_id = $2
ORDER BY hr.created_at DESC;

-- name: ResolveHealthReport :one
UPDATE pilgrim_health_reports
SET resolved = true, resolved_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: GetHealthReportForOperator :one
SELECT * FROM pilgrim_health_reports WHERE id = $1 AND operator_id = $2;

-- name: HasOpenSevereHealthReport :one
-- Business rule: a pilgrim with an active BERAT report can't be swept into
-- a bulk journey-status update — see JourneyService.bulkUpdateStatus.
SELECT EXISTS(
  SELECT 1 FROM pilgrim_health_reports
  WHERE pilgrim_id = $1 AND operator_id = $2 AND resolved = false AND severity = 'BERAT'
) AS has_open_severe;
