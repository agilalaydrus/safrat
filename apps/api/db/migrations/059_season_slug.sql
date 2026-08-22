-- +goose Up
ALTER TABLE seasons ADD COLUMN slug TEXT;

-- Backfill: full season name slugified (unlike operators, where only the
-- first word is used — a season's full name is what actually distinguishes
-- it, e.g. "Musim Haji 2025" vs "Musim Haji 2026"), scoped unique per
-- operator (not globally, since the operator is already established by the
-- subdomain). Ties within the same operator get a numeric suffix.
WITH base AS (
  SELECT id, operator_id,
         NULLIF(regexp_replace(regexp_replace(lower(trim(name)), '[^a-z0-9]+', '-', 'g'), '(^-|-$)', '', 'g'), '') AS candidate,
         ROW_NUMBER() OVER (
           PARTITION BY operator_id, regexp_replace(regexp_replace(lower(trim(name)), '[^a-z0-9]+', '-', 'g'), '(^-|-$)', '', 'g')
           ORDER BY created_at
         ) AS rank
  FROM seasons
)
UPDATE seasons s
SET slug = CASE WHEN base.rank = 1 THEN base.candidate ELSE base.candidate || '-' || base.rank END
FROM base
WHERE s.id = base.id AND base.candidate IS NOT NULL;

CREATE UNIQUE INDEX seasons_operator_slug_key ON seasons (operator_id, slug) WHERE slug IS NOT NULL;

-- +goose Down
DROP INDEX seasons_operator_slug_key;
ALTER TABLE seasons DROP COLUMN slug;
