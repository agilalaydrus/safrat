-- +goose Up
-- Kloter is the real administrative unit Indonesian Hajj/Umrah operators
-- schedule departures around — a numbered embarkation batch tied to one
-- flight and one departure date (e.g. "SOC-01" departing from Soekarno-Hatta).
-- It's a separate concept from "groups" (rombongan): a kloter is the
-- Kemenag-level departure batch, a rombongan is the pastoral/logistics
-- subdivision a travel agency's own ketua rombongan leads within it. One
-- kloter typically contains several rombongan.
CREATE TABLE kloters (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id       UUID        NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  code            TEXT        NOT NULL,
  embarkation     TEXT        NOT NULL DEFAULT '',
  flight_number   TEXT        NOT NULL DEFAULT '',
  departure_date  TIMESTAMPTZ,
  capacity        INT         NOT NULL DEFAULT 0 CHECK (capacity >= 0),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, code)
);
CREATE INDEX kloters_operator_season_idx ON kloters(operator_id, season_id);

ALTER TABLE pilgrims
  ADD COLUMN kloter_id UUID REFERENCES kloters(id) ON DELETE SET NULL;
CREATE INDEX pilgrims_kloter_id_idx ON pilgrims(kloter_id);

-- +goose Down
ALTER TABLE pilgrims DROP COLUMN IF EXISTS kloter_id;
DROP TABLE IF EXISTS kloters;
