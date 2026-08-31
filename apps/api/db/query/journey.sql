-- name: UpsertPilgrimJourneyStatus :one
INSERT INTO pilgrim_journey_status (operator_id, pilgrim_id, status, updated_by, notes)
SELECT sqlc.arg(operator_id), p.id, sqlc.arg(status), sqlc.arg(updated_by), sqlc.arg(notes)
FROM pilgrims p
WHERE p.id = sqlc.arg(pilgrim_id) AND p.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ON CONFLICT (pilgrim_id) DO UPDATE SET status = sqlc.arg(status), updated_by = sqlc.arg(updated_by), notes = sqlc.arg(notes), updated_at = NOW()
RETURNING *;

-- name: InsertPilgrimJourneyLog :exec
INSERT INTO pilgrim_journey_log (operator_id, pilgrim_id, from_status, to_status, updated_by, notes)
SELECT sqlc.arg(operator_id), p.id, sqlc.arg(from_status), sqlc.arg(to_status), sqlc.arg(updated_by), sqlc.arg(notes)
FROM pilgrims p
WHERE p.id = sqlc.arg(pilgrim_id) AND p.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: GetPilgrimJourneyStatus :one
SELECT s.*, COALESCE(u.name, '') AS updated_by_name
FROM pilgrim_journey_status s
JOIN pilgrims p ON p.id = s.pilgrim_id AND p.operator_id = s.operator_id
LEFT JOIN "user" u ON u.id = s.updated_by
WHERE s.pilgrim_id = sqlc.arg(pilgrim_id) AND s.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: ListJourneyStatusesByKloter :many
-- One row per non-substituted pilgrim in the kloter — pilgrims with no
-- pilgrim_journey_status row yet (shouldn't normally happen after the
-- migration-069 backfill, but a pilgrim created after it and never opened
-- once) are treated as REGISTERED via COALESCE.
SELECT p.id AS pilgrim_id, COALESCE(s.status, 'REGISTERED') AS status
FROM pilgrims p
LEFT JOIN pilgrim_journey_status s ON s.pilgrim_id = p.id
WHERE p.operator_id = sqlc.arg(operator_id) AND p.kloter_id = sqlc.arg(kloter_id) AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: CountJourneyStatusesByKloter :many
SELECT COALESCE(s.status, 'REGISTERED') AS status, COUNT(*)::int AS count
FROM pilgrims p
LEFT JOIN pilgrim_journey_status s ON s.pilgrim_id = p.id
WHERE p.operator_id = sqlc.arg(operator_id) AND p.kloter_id = sqlc.arg(kloter_id) AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
GROUP BY COALESCE(s.status, 'REGISTERED');
