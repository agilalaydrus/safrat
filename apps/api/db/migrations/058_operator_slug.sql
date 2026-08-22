-- +goose Up
ALTER TABLE operators ADD COLUMN slug TEXT;

-- Backfill existing operators: first word of their name, lowercased,
-- stripped to [a-z0-9]. Ties (two operators whose first word collides)
-- get a numeric suffix based on row order — arbitrary but deterministic,
-- and fine since this only affects pre-existing rows at migration time;
-- new operators get a proper uniqueness-checked slug from CreateOperator.
WITH base AS (
  SELECT id,
         NULLIF(regexp_replace(lower(split_part(name, ' ', 1)), '[^a-z0-9]', '', 'g'), '') AS candidate,
         ROW_NUMBER() OVER (
           PARTITION BY regexp_replace(lower(split_part(name, ' ', 1)), '[^a-z0-9]', '', 'g')
           ORDER BY created_at
         ) AS rank
  FROM operators
)
UPDATE operators o
SET slug = CASE WHEN base.rank = 1 THEN base.candidate ELSE base.candidate || '-' || base.rank END
FROM base
WHERE o.id = base.id AND base.candidate IS NOT NULL;

CREATE UNIQUE INDEX operators_slug_key ON operators (slug) WHERE slug IS NOT NULL;

-- +goose Down
DROP INDEX operators_slug_key;
ALTER TABLE operators DROP COLUMN slug;
