-- name: CreateAddonItem :one
INSERT INTO addon_items (operator_id, season_id, name, unit_price_idr)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateAddonItem :one
UPDATE addon_items
SET name = $3, unit_price_idr = $4, is_active = $5
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: ListAddonItems :many
SELECT * FROM addon_items WHERE operator_id = $1 AND season_id = $2 ORDER BY name;

-- name: GetAddonItem :one
SELECT * FROM addon_items WHERE id = $1 AND operator_id = $2;

-- name: AssignPilgrimAddon :one
INSERT INTO pilgrim_addons (operator_id, pilgrim_id, addon_item_id, quantity, unit_price_idr, notes)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (pilgrim_id, addon_item_id) DO UPDATE
  SET quantity = EXCLUDED.quantity, unit_price_idr = EXCLUDED.unit_price_idr, notes = EXCLUDED.notes
RETURNING *;

-- name: SetPilgrimAddonPaid :one
UPDATE pilgrim_addons SET paid = $3 WHERE id = $1 AND operator_id = $2 RETURNING *;

-- name: DeletePilgrimAddon :exec
DELETE FROM pilgrim_addons WHERE id = $1 AND operator_id = $2;

-- name: ListPilgrimAddons :many
-- group_id narrows to one group's roster ("jamaah ber-add-on di grup ini");
-- left empty it returns every assignment in the season.
SELECT
  pa.id, pa.pilgrim_id, pa.addon_item_id, pa.quantity, pa.unit_price_idr,
  (pa.quantity * pa.unit_price_idr)::bigint AS total_idr,
  pa.paid, pa.notes, pa.created_at,
  p.full_name AS pilgrim_name, COALESCE(g.name, '') AS group_name,
  ai.name AS addon_name
FROM pilgrim_addons pa
JOIN pilgrims p ON p.id = pa.pilgrim_id
JOIN addon_items ai ON ai.id = pa.addon_item_id
LEFT JOIN groups g ON g.id = p.group_id
WHERE pa.operator_id = $1 AND p.season_id = $2
  AND (sqlc.narg(group_id)::uuid IS NULL OR p.group_id = sqlc.narg(group_id))
ORDER BY p.full_name, ai.name;
