-- name: ListActiveSOSForSeason :many
SELECT s.*, p.full_name AS pilgrim_name, g.name AS group_name, g.id AS group_id
FROM sos_alerts s
JOIN pilgrims p ON p.id = s.pilgrim_id
LEFT JOIN groups g ON g.id = p.group_id
WHERE s.operator_id = $1 AND p.season_id = $2 AND s.status IN ('ACTIVE','ACKNOWLEDGED','ESCALATED')
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY s.created_at DESC;

-- name: ListOpenHealthReportsForSeason :many
SELECT hr.*, p.full_name AS pilgrim_name, g.name AS group_name
FROM pilgrim_health_reports hr
JOIN pilgrims p ON p.id = hr.pilgrim_id
JOIN groups g ON g.id = hr.group_id
WHERE hr.operator_id = $1 AND p.season_id = $2 AND hr.resolved = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY hr.created_at DESC;

-- name: ListGroupRitualProgressForSeason :many
-- template_count/pilgrim_count/completed_count let the caller compute a
-- per-group ritual completion % without an extra round trip per group —
-- template_count is 0 (no ritual templates seeded for this season_type
-- yet) is a valid, common state, not an error.
SELECT g.id AS group_id,
  COUNT(DISTINCT rt.id)::int AS template_count,
  COUNT(DISTINCT p.id)::int AS pilgrim_count,
  COUNT(pr.id) FILTER (WHERE pr.completed)::int AS completed_count
FROM groups g
JOIN seasons s ON s.id = g.season_id
LEFT JOIN ritual_templates rt ON rt.operator_id = g.operator_id
  AND rt.season_type = (CASE WHEN s.type = 'HAJJ' THEN 'HAJJ' ELSE 'UMRAH' END)
LEFT JOIN pilgrims p ON p.group_id = g.id AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
LEFT JOIN pilgrim_rituals pr ON pr.pilgrim_id = p.id AND pr.ritual_id = rt.id
WHERE g.operator_id = $1 AND g.season_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR EXISTS (SELECT 1 FROM pilgrims gp WHERE gp.group_id=g.id AND gp.branch_id=sqlc.narg(branch_scope)::uuid))
GROUP BY g.id;

-- name: ListReturnTimelineForSeason :many
-- Kloters with a RETURN flight leg scheduled in the next 7 days —
-- "ready_count" is a proxy (journey status already at/past
-- PRE_DEPARTURE_SAUDI) until a real per-pilgrim boarding checklist exists.
SELECT k.id AS kloter_id, k.code AS kloter_code, m.scheduled_at AS return_at,
  COUNT(DISTINCT p.id)::int AS total_pilgrims,
  COUNT(DISTINCT p.id) FILTER (WHERE js.status IN ('PRE_DEPARTURE_SAUDI','DEPARTED_SAUDI','IN_TRANSIT_RETURN','ARRIVED_INDONESIA','COMPLETED'))::int AS ready_count
FROM kloters k
JOIN movements m ON m.kloter_id = k.id AND m.mode = 'FLIGHT' AND m.trip_leg = 'RETURN'
LEFT JOIN pilgrims p ON p.kloter_id = k.id AND p.is_substituted = false
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
LEFT JOIN pilgrim_journey_status js ON js.pilgrim_id = p.id
WHERE k.operator_id = $1 AND k.season_id = $2
  AND m.scheduled_at BETWEEN NOW() AND NOW() + INTERVAL '7 days'
GROUP BY k.id, k.code, m.scheduled_at
ORDER BY m.scheduled_at ASC;
