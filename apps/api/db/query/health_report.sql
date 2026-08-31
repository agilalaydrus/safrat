-- name: CreateHealthReport :one
INSERT INTO pilgrim_health_reports (operator_id, pilgrim_id, group_id, reported_by, severity, symptoms, action_taken)
SELECT sqlc.arg(operator_id), p.id, g.id, sqlc.arg(reported_by), sqlc.arg(severity), sqlc.arg(symptoms), sqlc.arg(action_taken)
FROM pilgrims p
JOIN groups g ON g.id = sqlc.arg(group_id) AND g.operator_id = sqlc.arg(operator_id)
WHERE p.id = sqlc.arg(pilgrim_id) AND p.operator_id = sqlc.arg(operator_id) AND p.group_id = g.id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING *;

-- name: ListHealthReports :many
SELECT hr.*, p.full_name AS pilgrim_name, g.name AS group_name
FROM pilgrim_health_reports hr
JOIN pilgrims p ON p.id = hr.pilgrim_id
JOIN groups g ON g.id = hr.group_id
WHERE hr.operator_id = $1 AND (sqlc.narg('resolved')::boolean IS NULL OR hr.resolved = sqlc.narg('resolved'))
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY hr.created_at DESC;

-- name: ListHealthReportsForGroup :many
SELECT hr.*, p.full_name AS pilgrim_name, g.name AS group_name
FROM pilgrim_health_reports hr
JOIN pilgrims p ON p.id = hr.pilgrim_id
JOIN groups g ON g.id = hr.group_id
WHERE hr.operator_id = $1 AND hr.group_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY hr.created_at DESC;

-- name: ResolveHealthReport :one
UPDATE pilgrim_health_reports hr
SET resolved = true, resolved_at = NOW()
FROM pilgrims p
WHERE hr.id = $1 AND hr.operator_id = $2 AND p.id = hr.pilgrim_id
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
RETURNING hr.*;

-- name: GetHealthReportForOperator :one
SELECT hr.*
FROM pilgrim_health_reports hr
JOIN pilgrims p ON p.id = hr.pilgrim_id
WHERE hr.id = $1 AND hr.operator_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: HasOpenSevereHealthReport :one
-- Business rule: a pilgrim with an active BERAT report can't be swept into
-- a bulk journey-status update — see JourneyService.bulkUpdateStatus.
SELECT EXISTS(
  SELECT 1
  FROM pilgrim_health_reports hr
  JOIN pilgrims p ON p.id = hr.pilgrim_id
  WHERE hr.pilgrim_id = $1 AND hr.operator_id = $2 AND hr.resolved = false AND hr.severity = 'BERAT'
    AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
) AS has_open_severe;
