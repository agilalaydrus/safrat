-- name: CreateSeason :one
INSERT INTO seasons (operator_id, name, type, start_date, end_date, capacity, slug)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (operator_id, lower(btrim(name)))
DO UPDATE SET name = seasons.name
WHERE seasons.type = EXCLUDED.type
  AND seasons.start_date = EXCLUDED.start_date
  AND seasons.end_date = EXCLUDED.end_date
  AND seasons.capacity = EXCLUDED.capacity
RETURNING *;

-- name: SeasonSlugExists :one
SELECT EXISTS(SELECT 1 FROM seasons WHERE operator_id = $1 AND slug = $2);

-- name: GetSeasonBySlug :one
SELECT * FROM seasons WHERE operator_id = $1 AND slug = $2;

-- name: ListSeasonsByOperatorID :many
SELECT * FROM seasons
WHERE operator_id = $1
ORDER BY start_date DESC;

-- name: GetSeasonByID :one
SELECT * FROM seasons
WHERE id = $1 AND operator_id = $2;

-- name: GetActiveSeasonIDForOperator :one
-- Nothing enforces at most one active season per operator (is_active is a
-- plain boolean, not a partial unique index) — in the rare case more than
-- one is active, most-recently-started wins, deterministically.
SELECT id FROM seasons
WHERE operator_id = $1 AND is_active = true
ORDER BY start_date DESC
LIMIT 1;

-- name: UpdateSeason :one
UPDATE seasons
SET name = $3, type = $4, start_date = $5, end_date = $6, capacity = $7
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteSeason :exec
DELETE FROM seasons WHERE id = $1 AND operator_id = $2;

-- name: SeasonHasData :one
-- Every one of these tables cascade-deletes on the season's FK — this is
-- the guard that stops DeleteSeason from silently wiping a season's whole
-- pilgrim/group/product/order history. Short-circuits on the first match.
-- season_id bound once via the CTE (repeating $1 as a raw placeholder
-- across every UNION branch trips sqlc's query analyzer).
WITH target AS (SELECT $1::uuid AS season_id)
SELECT EXISTS(
  SELECT 1 FROM pilgrims p, target t WHERE p.season_id = t.season_id
  UNION ALL SELECT 1 FROM groups g, target t WHERE g.season_id = t.season_id
  UNION ALL SELECT 1 FROM kloters k, target t WHERE k.season_id = t.season_id
  UNION ALL SELECT 1 FROM hotels h, target t WHERE h.season_id = t.season_id
  UNION ALL SELECT 1 FROM movements m, target t WHERE m.season_id = t.season_id
  UNION ALL SELECT 1 FROM products pr, target t WHERE pr.season_id = t.season_id
  UNION ALL SELECT 1 FROM orders o, target t WHERE o.season_id = t.season_id
) AS has_data;

-- name: SetActiveSeason :many
UPDATE seasons
SET is_active = (seasons.id = $1)
WHERE seasons.operator_id = $2
  AND EXISTS (
    SELECT 1 FROM seasons AS target
    WHERE target.id = $1 AND target.operator_id = $2
  )
RETURNING *;

-- name: ListPublicSeasonsByOperator :many
-- Public "available packages" for an operator's profile page: seasons that
-- haven't ended yet, with their registered-pilgrim count as social proof.
-- No is_registration_open column exists — end_date >= NOW() is the "still
-- available" signal. Only non-sensitive columns are selected.
SELECT s.id, s.name, s.type, s.start_date, s.end_date,
       COUNT(p.id) AS pilgrim_count
FROM seasons s
LEFT JOIN pilgrims p ON p.season_id = s.id
WHERE s.operator_id = $1 AND s.end_date >= NOW()
GROUP BY s.id, s.name, s.type, s.start_date, s.end_date
ORDER BY s.start_date ASC;
