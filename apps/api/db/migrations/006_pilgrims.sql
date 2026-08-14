-- +goose Up
CREATE TABLE pilgrims (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  season_id           UUID NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  operator_id         UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  group_id            UUID REFERENCES groups(id) ON DELETE SET NULL,
  full_name           TEXT NOT NULL,
  passport_number     TEXT NOT NULL,
  nationality         TEXT NOT NULL,
  date_of_birth       TIMESTAMPTZ NOT NULL,
  gender              TEXT NOT NULL CHECK (gender IN ('MALE', 'FEMALE')),
  photo_url           TEXT,
  phone               TEXT,
  emergency_contact   TEXT,
  preferred_lang      TEXT NOT NULL DEFAULT 'ar',
  medical_notes       TEXT,
  requires_wheelchair BOOLEAN NOT NULL DEFAULT FALSE,
  mahram_id           UUID REFERENCES pilgrims(id) ON DELETE SET NULL,
  is_substituted      BOOLEAN NOT NULL DEFAULT FALSE,
  substituted_by_id   UUID REFERENCES pilgrims(id) ON DELETE SET NULL,
  app_access_code     TEXT UNIQUE NOT NULL DEFAULT gen_random_uuid()::text,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(season_id, passport_number)
);

CREATE INDEX pilgrims_operator_season_idx ON pilgrims(operator_id, season_id);
CREATE INDEX pilgrims_group_id_idx ON pilgrims(group_id);
CREATE INDEX pilgrims_app_access_code_idx ON pilgrims(app_access_code);

CREATE TRIGGER pilgrims_set_updated_at
BEFORE UPDATE ON pilgrims
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS pilgrims;
