-- +goose Up
-- An onboarding retry used to create the same season more than once. Keep one
-- canonical row for each exact business key and remove only empty duplicates.
-- If two duplicate rows both own data, abort rather than silently cascading
-- that data away; such a case requires an explicit manual merge.
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
      PARTITION BY operator_id, lower(btrim(name)), type, start_date, end_date, capacity
      ORDER BY has_data DESC, is_active DESC, created_at, id
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
    GROUP BY operator_id, lower(btrim(name)), type, start_date, end_date, capacity
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'duplicate seasons with dependent data require manual merge before migration 077';
  END IF;
END $$;
-- +goose StatementEnd

CREATE UNIQUE INDEX seasons_exact_business_key_unique
  ON seasons (operator_id, lower(btrim(name)), type, start_date, end_date, capacity);

-- +goose Down
DROP INDEX IF EXISTS seasons_exact_business_key_unique;
