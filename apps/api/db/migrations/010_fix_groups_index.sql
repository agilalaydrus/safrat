-- +goose Up
CREATE INDEX IF NOT EXISTS groups_operator_season_idx ON groups(operator_id, season_id);
CREATE INDEX IF NOT EXISTS groups_leader_idx ON groups(leader_id) WHERE leader_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS groups_operator_season_idx;
DROP INDEX IF EXISTS groups_leader_idx;
