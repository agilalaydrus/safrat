-- +goose Up
-- Migration 058 derived operator slugs from the first word only. That made
-- common Indonesian legal names such as "PT Vacana Indonesia" resolve to the
-- generic and collision-prone "pt". Repair only those generic slugs; existing
-- meaningful public links remain unchanged.
-- +goose StatementBegin
DO $$
DECLARE
  operator_row RECORD;
  base_slug TEXT;
  candidate TEXT;
  suffix_number INTEGER;
  suffix_text TEXT;
BEGIN
  FOR operator_row IN
    SELECT id, name
    FROM operators
    WHERE slug ~ '^(pt|cv|ud|pd|fa|kbih|kbihu|yayasan)(-[0-9]+)?$'
    ORDER BY created_at, id
  LOOP
    base_slug := lower(trim(operator_row.name));
    base_slug := regexp_replace(
      base_slug,
      '^[[:space:]]*(pt|cv|ud|pd|fa|kbih|kbihu|yayasan)[[:space:].,_-]+',
      '',
      'i'
    );
    base_slug := trim(BOTH '-' FROM regexp_replace(base_slug, '[^a-z0-9]+', '-', 'g'));
    base_slug := trim(TRAILING '-' FROM left(base_slug, 55));

    -- A legal prefix with no brand name cannot be made more meaningful.
    IF base_slug = '' THEN
      CONTINUE;
    END IF;

    candidate := base_slug;
    suffix_number := 1;
    WHILE EXISTS (
      SELECT 1 FROM operators
      WHERE slug = candidate AND id <> operator_row.id
    ) OR candidate IN ('app', 'api', 'www') LOOP
      suffix_number := suffix_number + 1;
      suffix_text := '-' || suffix_number::TEXT;
      candidate := trim(TRAILING '-' FROM left(base_slug, 63 - length(suffix_text))) || suffix_text;
    END LOOP;

    UPDATE operators SET slug = candidate WHERE id = operator_row.id;
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Intentionally irreversible: restoring generic slugs such as "pt" would
-- recreate the production bug and could collide with later operators.
SELECT 1;
