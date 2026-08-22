-- name: UpsertPilgrimJourneyStatus :one
INSERT INTO pilgrim_journey_status (operator_id, pilgrim_id, status, updated_by, notes)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (pilgrim_id) DO UPDATE SET status = $3, updated_by = $4, notes = $5, updated_at = NOW()
RETURNING *;

-- name: InsertPilgrimJourneyLog :exec
INSERT INTO pilgrim_journey_log (operator_id, pilgrim_id, from_status, to_status, updated_by, notes)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetPilgrimJourneyStatus :one
SELECT s.*, COALESCE(u.name, '') AS updated_by_name
FROM pilgrim_journey_status s
LEFT JOIN "user" u ON u.id = s.updated_by
WHERE s.pilgrim_id = $1 AND s.operator_id = $2;

-- name: ListJourneyStatusesByKloter :many
-- One row per non-substituted pilgrim in the kloter — pilgrims with no
-- pilgrim_journey_status row yet (shouldn't normally happen after the
-- migration-069 backfill, but a pilgrim created after it and never opened
-- once) are treated as REGISTERED via COALESCE.
SELECT p.id AS pilgrim_id, COALESCE(s.status, 'REGISTERED') AS status
FROM pilgrims p
LEFT JOIN pilgrim_journey_status s ON s.pilgrim_id = p.id
WHERE p.operator_id = $1 AND p.kloter_id = $2 AND p.is_substituted = false;

-- name: CountJourneyStatusesByKloter :many
SELECT COALESCE(s.status, 'REGISTERED') AS status, COUNT(*)::int AS count
FROM pilgrims p
LEFT JOIN pilgrim_journey_status s ON s.pilgrim_id = p.id
WHERE p.operator_id = $1 AND p.kloter_id = $2 AND p.is_substituted = false
GROUP BY COALESCE(s.status, 'REGISTERED');
