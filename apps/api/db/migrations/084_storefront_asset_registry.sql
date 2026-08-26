-- +goose Up
CREATE TABLE operator_storefront_assets (
  reservation_key TEXT PRIMARY KEY,
  object_key      TEXT UNIQUE,
  operator_id    UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  kind           VARCHAR(40) NOT NULL,
  size_bytes     BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 10485760),
  state          VARCHAR(16) NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING', 'LIVE')),
  public_url     TEXT,
  reserved_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at     TIMESTAMPTZ NOT NULL,
  confirmed_at   TIMESTAMPTZ,
  orphaned_at    TIMESTAMPTZ,
  CHECK ((state = 'PENDING' AND object_key IS NULL AND public_url IS NULL AND confirmed_at IS NULL) OR
         (state = 'LIVE' AND object_key IS NOT NULL AND public_url IS NOT NULL AND confirmed_at IS NOT NULL))
);

CREATE INDEX operator_storefront_assets_usage_idx
  ON operator_storefront_assets (operator_id, state, expires_at);

CREATE INDEX operator_storefront_assets_gc_idx
  ON operator_storefront_assets (orphaned_at)
  WHERE state = 'LIVE' AND orphaned_at IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS operator_storefront_assets;
