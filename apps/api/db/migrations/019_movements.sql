-- +goose Up
CREATE TABLE movements (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  season_id UUID NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  name TEXT NOT NULL, origin TEXT NOT NULL, destination TEXT NOT NULL,
  scheduled_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','departed','arrived','cancelled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX movements_operator_season_idx ON movements(operator_id, season_id);
CREATE INDEX movements_scheduled_at_idx ON movements(scheduled_at);
CREATE TRIGGER movements_set_updated_at BEFORE UPDATE ON movements FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose Down
DROP TABLE IF EXISTS movements;
