-- +goose Up
ALTER TABLE pilgrim_registrations
  ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS pilgrim_registrations_agent_idx ON pilgrim_registrations(agent_id);

-- +goose Down
DROP INDEX IF EXISTS pilgrim_registrations_agent_idx;
ALTER TABLE pilgrim_registrations DROP COLUMN IF EXISTS agent_id;
