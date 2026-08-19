-- +goose Up
CREATE TABLE pilgrim_registrations (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  product_id    UUID        REFERENCES products(id) ON DELETE SET NULL,
  full_name     TEXT        NOT NULL,
  passport_number TEXT      NOT NULL DEFAULT '',
  date_of_birth DATE,
  gender        TEXT        NOT NULL DEFAULT 'MALE' CHECK (gender IN ('MALE','FEMALE')),
  phone         TEXT        NOT NULL DEFAULT '',
  email         TEXT        NOT NULL DEFAULT '',
  nationality   TEXT        NOT NULL DEFAULT 'IDN',
  address       TEXT        NOT NULL DEFAULT '',
  status        TEXT        NOT NULL DEFAULT 'PENDING'
    CHECK (status IN ('PENDING','APPROVED','REJECTED')),
  notes         TEXT        NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX pilgrim_registrations_operator_idx ON pilgrim_registrations(operator_id, season_id);
CREATE TRIGGER pilgrim_registrations_set_updated_at
  BEFORE UPDATE ON pilgrim_registrations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS pilgrim_registrations;
