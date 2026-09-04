-- name: CreateAgendaEvent :one
INSERT INTO agenda_events (operator_id, branch_id, season_id, title, location, starts_at, ends_at, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateAgendaEvent :one
UPDATE agenda_events
SET branch_id = $3, season_id = $4, title = $5, location = $6, starts_at = $7, ends_at = $8, notes = $9
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteAgendaEvent :exec
DELETE FROM agenda_events WHERE id = $1 AND operator_id = $2;

-- name: ListAgendaEvents :many
-- Events with no season (NULL) show on every season's agenda; events with no
-- branch (NULL) are head office and show regardless of the branch filter.
SELECT e.*, COALESCE(b.name, '') AS branch_name
FROM agenda_events e
LEFT JOIN branches b ON b.id = e.branch_id
WHERE e.operator_id = $1
  AND (e.season_id = $2 OR e.season_id IS NULL)
  AND (sqlc.narg(branch_id)::uuid IS NULL OR e.branch_id = sqlc.narg(branch_id))
ORDER BY e.starts_at;

-- name: ListAgendaManasikSessions :many
SELECT s.id, s.title, s.location, s.scheduled_at, s.duration_minutes,
       COALESCE(k.code, '') AS kloter_code, s.status
FROM manasik_sessions s
LEFT JOIN kloters k ON k.id = s.kloter_id
WHERE s.operator_id = $1 AND s.season_id = $2
ORDER BY s.scheduled_at;

-- name: ListAgendaKloterMovements :many
-- A kloter's spine always starts and ends on a TRANSPORT segment (migration
-- 155's constraint). The first is departure, the last is return — skipped
-- when they are the same segment, i.e. the itinerary has only one leg built
-- so far, which would otherwise double-count it as both.
WITH bounds AS (
  SELECT kloter_id, MIN(position) AS first_pos, MAX(position) AS last_pos
  FROM kloter_itinerary_segments
  WHERE segment_type = 'TRANSPORT'
  GROUP BY kloter_id
  HAVING MIN(position) <> MAX(position)
)
SELECT
  k.id AS kloter_id, k.code AS kloter_code,
  CASE WHEN s.position = b.first_pos THEN 'DEPARTURE' ELSE 'RETURN' END::text AS leg,
  m.name AS movement_name, m.origin, m.destination, m.scheduled_at
FROM kloter_itinerary_segments s
JOIN bounds b ON b.kloter_id = s.kloter_id AND s.position IN (b.first_pos, b.last_pos)
JOIN kloters k ON k.id = s.kloter_id
JOIN movements m ON m.id = s.movement_id
WHERE k.operator_id = $1 AND k.season_id = $2
ORDER BY m.scheduled_at;
