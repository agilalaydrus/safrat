-- +goose Up
CREATE TABLE broadcasts (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id   UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  title       TEXT        NOT NULL,
  body        TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX broadcasts_operator_season_idx ON broadcasts(operator_id, season_id);
CREATE TRIGGER broadcasts_set_updated_at
  BEFORE UPDATE ON broadcasts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS broadcasts;
