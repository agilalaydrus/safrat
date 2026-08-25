-- +goose Up
-- Keep current and likely future asset service hostnames unavailable to
-- tenants. Exact Nginx service hosts must always win over wildcard storefronts.
-- +goose StatementBegin
DO $$
DECLARE
  conflicting_slugs TEXT;
BEGIN
  SELECT string_agg(slug, ', ' ORDER BY slug)
  INTO conflicting_slugs
  FROM operators
  WHERE slug IN ('assets', 'cdn', 'media', 'storage');

  IF conflicting_slugs IS NOT NULL THEN
    RAISE EXCEPTION
      'operator slugs conflict with reserved storage hostnames: %. Rename them before migration 083',
      conflicting_slugs;
  END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE operators DROP CONSTRAINT operators_slug_policy_check;

ALTER TABLE operators
  ADD CONSTRAINT operators_slug_policy_check
  CHECK (
    slug IS NULL OR (
      length(slug) BETWEEN 3 AND 63 AND
      slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$' AND
      slug NOT IN (
        'admin', 'api', 'app', 'assets', 'auth', 'cdn', 'dashboard',
        'docs', 'help', 'media', 'status', 'storage', 'support', 'www'
      )
    )
  );

-- +goose Down
ALTER TABLE operators DROP CONSTRAINT operators_slug_policy_check;

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
