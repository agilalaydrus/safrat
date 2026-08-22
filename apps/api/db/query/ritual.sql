-- name: ListRitualTemplates :many
SELECT * FROM ritual_templates
WHERE operator_id = $1 AND season_type = $2
ORDER BY order_num ASC;

-- name: CreateRitualTemplate :one
INSERT INTO ritual_templates (operator_id, season_type, name, description, order_num, is_required)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CountRitualTemplates :one
SELECT COUNT(*)::int FROM ritual_templates WHERE operator_id = $1 AND season_type = $2;

-- name: UpsertPilgrimRitual :one
INSERT INTO pilgrim_rituals (operator_id, pilgrim_id, ritual_id, completed, completed_at, completed_by, notes)
VALUES ($1, $2, $3, true, NOW(), $4, $5)
ON CONFLICT (pilgrim_id, ritual_id) DO UPDATE SET completed = true, completed_at = NOW(), completed_by = $4, notes = $5
RETURNING *;

-- name: GetPilgrimRitualStatus :one
SELECT rt.id AS ritual_id, rt.name, rt.order_num, rt.is_required,
  COALESCE(pr.completed, false) AS completed, pr.completed_at, COALESCE(u.name, '') AS completed_by_name
FROM ritual_templates rt
LEFT JOIN pilgrim_rituals pr ON pr.ritual_id = rt.id AND pr.pilgrim_id = $2
LEFT JOIN "user" u ON u.id = pr.completed_by
WHERE rt.operator_id = $1 AND rt.season_type = $3
ORDER BY rt.order_num ASC;

-- name: GetPilgrimRitualStatusByAccessCode :many
-- Public counterpart of GetPilgrimRitualStatus for the pilgrim app — auth
-- by app_access_code, mirrors the same "resolve season_type via the
-- pilgrim's own season" pattern as CountRitualCompletionByGroup.
SELECT rt.id AS ritual_id, rt.name, rt.description, rt.order_num, rt.is_required,
  COALESCE(pr.completed, false) AS completed, pr.completed_at, COALESCE(u.name, '') AS completed_by_name
FROM pilgrims p
JOIN seasons s ON s.id = p.season_id
JOIN ritual_templates rt ON rt.operator_id = p.operator_id
  AND rt.season_type = (CASE WHEN s.type = 'HAJJ' THEN 'HAJJ' ELSE 'UMRAH' END)
LEFT JOIN pilgrim_rituals pr ON pr.ritual_id = rt.id AND pr.pilgrim_id = p.id
LEFT JOIN "user" u ON u.id = pr.completed_by
WHERE p.app_access_code = $1 AND p.is_substituted = false
ORDER BY rt.order_num ASC;

-- name: CountRitualCompletionByGroup :many
SELECT rt.id AS ritual_id, rt.name, rt.order_num,
  COUNT(DISTINCT p.id)::int AS total_pilgrims,
  COUNT(DISTINCT pr.pilgrim_id) FILTER (WHERE pr.completed)::int AS completed_count
FROM groups g
JOIN seasons s ON s.id = g.season_id
JOIN ritual_templates rt ON rt.operator_id = g.operator_id
  AND rt.season_type = (CASE WHEN s.type = 'HAJJ' THEN 'HAJJ' ELSE 'UMRAH' END)
JOIN pilgrims p ON p.group_id = g.id AND p.is_substituted = false
LEFT JOIN pilgrim_rituals pr ON pr.ritual_id = rt.id AND pr.pilgrim_id = p.id
WHERE g.id = $1 AND g.operator_id = $2
GROUP BY rt.id, rt.name, rt.order_num
ORDER BY rt.order_num ASC;

-- name: ListIncompletePilgrimNamesForRitual :many
SELECT p.id, p.full_name FROM pilgrims p
WHERE p.group_id = $1 AND p.operator_id = $2 AND p.is_substituted = false
  AND NOT EXISTS (SELECT 1 FROM pilgrim_rituals pr WHERE pr.pilgrim_id = p.id AND pr.ritual_id = $3 AND pr.completed = true)
ORDER BY p.full_name ASC;

-- name: SeasonTypeBucket :one
SELECT (CASE WHEN type = 'HAJJ' THEN 'HAJJ' ELSE 'UMRAH' END)::text AS bucket
FROM seasons WHERE id = $1;
