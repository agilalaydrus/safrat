-- name: CreateChecklistTemplate :one
INSERT INTO checklist_templates (operator_id, season_id, title, description, category, is_required, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListChecklistTemplates :many
SELECT * FROM checklist_templates
WHERE operator_id = $1 AND season_id = $2
ORDER BY sort_order ASC;

-- name: DeleteChecklistTemplate :exec
DELETE FROM checklist_templates WHERE id = $1 AND operator_id = $2;

-- name: UpsertPilgrimChecklistItem :one
INSERT INTO pilgrim_checklist_items (template_id, pilgrim_id, operator_id, is_completed, completed_at, completed_by, notes)
SELECT $1, $2, $3, $4,
       CASE WHEN $4 THEN NOW() ELSE NULL END, $5, $6
FROM checklist_templates ct
JOIN pilgrims p ON p.id = $2 AND p.operator_id = $3
WHERE ct.id = $1 AND ct.operator_id = $3
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ON CONFLICT (template_id, pilgrim_id) DO UPDATE
  SET is_completed = EXCLUDED.is_completed,
      completed_at = CASE WHEN EXCLUDED.is_completed THEN NOW() ELSE NULL END,
      completed_by = EXCLUDED.completed_by,
      notes        = EXCLUDED.notes
RETURNING *;

-- name: GetPilgrimChecklist :many
-- Returns all template items for a pilgrim with completion state — LEFT
-- JOIN so a template item with no pilgrim_checklist_items row yet still
-- shows up as incomplete instead of being silently missing.
SELECT
  ct.id AS template_id, ct.title, ct.description, ct.category, ct.is_required, ct.sort_order,
  COALESCE(pci.is_completed, false) AS is_completed,
  pci.completed_at, pci.completed_by, pci.notes
FROM checklist_templates ct
LEFT JOIN pilgrim_checklist_items pci
  ON pci.template_id = ct.id AND pci.pilgrim_id = $3
WHERE ct.operator_id = $1 AND ct.season_id = $2
  AND EXISTS (
    SELECT 1 FROM pilgrims p
    WHERE p.id = $3 AND p.operator_id = $1 AND p.season_id = $2
      AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
  )
ORDER BY ct.sort_order ASC;

-- name: GetChecklistCompletionStats :many
-- Operator dashboard: completion rate per template item across all
-- active pilgrims in the season.
SELECT
  ct.id, ct.title, ct.category, ct.is_required,
  COUNT(pci.id) FILTER (WHERE pci.is_completed) AS completed_count,
  COUNT(p.id) AS total_pilgrims
FROM checklist_templates ct
CROSS JOIN pilgrims p
LEFT JOIN pilgrim_checklist_items pci
  ON pci.template_id = ct.id AND pci.pilgrim_id = p.id
WHERE ct.operator_id = $1
  AND ct.season_id   = $2
  AND p.operator_id  = $1
  AND p.season_id    = $2
  AND p.status       = 'ACTIVE'
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
GROUP BY ct.id, ct.title, ct.category, ct.is_required, ct.sort_order
ORDER BY ct.sort_order;
