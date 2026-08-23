-- +goose Up
-- A season name is its operator-facing identity. Two rows with the same name
-- but slightly different dates are indistinguishable across selectors and
-- cards; detail changes belong in UpdateSeason, not another CreateSeason.
DROP INDEX IF EXISTS seasons_exact_business_key_unique;

WITH candidates AS (
  SELECT
    s.*,
    (
      EXISTS (SELECT 1 FROM pilgrims p WHERE p.season_id = s.id) OR
      EXISTS (SELECT 1 FROM groups g WHERE g.season_id = s.id) OR
      EXISTS (SELECT 1 FROM kloters k WHERE k.season_id = s.id) OR
      EXISTS (SELECT 1 FROM hotels h WHERE h.season_id = s.id) OR
      EXISTS (SELECT 1 FROM movements m WHERE m.season_id = s.id) OR
      EXISTS (SELECT 1 FROM products p WHERE p.season_id = s.id) OR
      EXISTS (SELECT 1 FROM orders o WHERE o.season_id = s.id) OR
      EXISTS (SELECT 1 FROM vendor_contracts vc WHERE vc.season_id = s.id) OR
      EXISTS (SELECT 1 FROM vendor_payments vp WHERE vp.season_id = s.id) OR
      EXISTS (SELECT 1 FROM season_waitlists sw WHERE sw.season_id = s.id) OR
      EXISTS (SELECT 1 FROM broadcasts b WHERE b.season_id = s.id) OR
      EXISTS (SELECT 1 FROM pilgrim_registrations pr WHERE pr.season_id = s.id) OR
      EXISTS (SELECT 1 FROM cancellation_policies cp WHERE cp.season_id = s.id) OR
      EXISTS (SELECT 1 FROM pilgrim_cancellations pc WHERE pc.season_id = s.id) OR
      EXISTS (SELECT 1 FROM checklist_templates ct WHERE ct.season_id = s.id)
    ) AS has_data
  FROM seasons s
), ranked AS (
  SELECT
    id,
    has_data,
    row_number() OVER (
      PARTITION BY operator_id, lower(btrim(name))
      -- With no dependent data, the latest submission represents the final
      -- onboarding values. A row with data or active status always wins.
      ORDER BY has_data DESC, is_active DESC, created_at DESC, id DESC
    ) AS duplicate_rank
  FROM candidates
)
DELETE FROM seasons s
USING ranked r
WHERE s.id = r.id
  AND r.duplicate_rank > 1
  AND NOT r.has_data;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM seasons
    GROUP BY operator_id, lower(btrim(name))
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'same-name seasons with dependent data require manual merge before migration 078';
  END IF;
END $$;
-- +goose StatementEnd

-- If the retained row previously got a numeric slug suffix only because an
-- empty duplicate owned the clean slug, reclaim that clean slug now.
WITH slug_candidates AS (
  SELECT
    id,
    operator_id,
    slug,
    trim(BOTH '-' FROM regexp_replace(lower(btrim(name)), '[^a-z0-9]+', '-', 'g')) AS base_slug
  FROM seasons
)
UPDATE seasons s
SET slug = candidate.base_slug
FROM slug_candidates candidate
WHERE s.id = candidate.id
  AND candidate.base_slug <> ''
  AND candidate.slug ~ ('^' || candidate.base_slug || '-[0-9]+$')
  AND NOT EXISTS (
    SELECT 1 FROM seasons existing
    WHERE existing.operator_id = candidate.operator_id
      AND existing.slug = candidate.base_slug
      AND existing.id <> candidate.id
  );

CREATE UNIQUE INDEX seasons_operator_normalized_name_unique
  ON seasons (operator_id, lower(btrim(name)));

-- +goose Down
DROP INDEX IF EXISTS seasons_operator_normalized_name_unique;
CREATE UNIQUE INDEX seasons_exact_business_key_unique
  ON seasons (operator_id, lower(btrim(name)), type, start_date, end_date, capacity);
