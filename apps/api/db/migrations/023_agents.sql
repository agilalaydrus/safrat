-- +goose Up
CREATE TABLE agents (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  name            TEXT        NOT NULL,
  phone           TEXT        NOT NULL DEFAULT '',
  email           TEXT        NOT NULL DEFAULT '',
  commission_rate FLOAT8      NOT NULL DEFAULT 0,
  notes           TEXT        NOT NULL DEFAULT '',
  is_active       BOOLEAN     NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX agents_operator_idx ON agents(operator_id);
CREATE TRIGGER agents_set_updated_at
  BEFORE UPDATE ON agents
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS pilgrims_agent_idx ON pilgrims(agent_id);

-- +goose Down
ALTER TABLE pilgrims DROP COLUMN IF EXISTS agent_id;
DROP TABLE IF EXISTS agents;
