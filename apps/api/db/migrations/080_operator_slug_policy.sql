-- +goose Up
-- Wildcard DNS makes every valid first-level label publicly reachable. Keep
-- platform/service names unavailable to tenants and enforce the same DNS label
-- policy at the database boundary, not only in onboarding code.
-- +goose StatementBegin
DO $$
DECLARE
  conflicting_slugs TEXT;
BEGIN
  SELECT string_agg(slug, ', ' ORDER BY slug)
  INTO conflicting_slugs
  FROM operators
  WHERE slug IS NOT NULL
    AND (
      length(slug) < 3 OR
      length(slug) > 63 OR
      slug !~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$' OR
      slug IN (
        'admin', 'api', 'app', 'auth', 'dashboard',
        'docs', 'help', 'status', 'support', 'www'
      )
    );

  IF conflicting_slugs IS NOT NULL THEN
    RAISE EXCEPTION
      'operator slugs violate wildcard hostname policy: %. Rename them before migration 080',
      conflicting_slugs;
  END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE operators
  ADD CONSTRAINT operators_slug_policy_check
  CHECK (
    slug IS NULL OR (
      length(slug) BETWEEN 3 AND 63 AND
      slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$' AND
      slug NOT IN (
        'admin', 'api', 'app', 'auth', 'dashboard',
        'docs', 'help', 'status', 'support', 'www'
      )
    )
  );

-- +goose Down
ALTER TABLE operators DROP CONSTRAINT IF EXISTS operators_slug_policy_check;
