-- +goose Up
CREATE TABLE products (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  name          TEXT        NOT NULL,
  type          TEXT        NOT NULL DEFAULT 'HAJJ' CHECK (type IN ('HAJJ','UMRAH')),
  price_idr     BIGINT      NOT NULL DEFAULT 0,
  duration_days INT         NOT NULL DEFAULT 0,
  description   TEXT        NOT NULL DEFAULT '',
  inclusions    TEXT[]      NOT NULL DEFAULT '{}',
  is_active     BOOLEAN     NOT NULL DEFAULT true,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX products_operator_season_idx ON products(operator_id, season_id);
CREATE TRIGGER products_set_updated_at
  BEFORE UPDATE ON products
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS products;
