-- +goose Up
ALTER TABLE groups ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE TRIGGER groups_updated_at BEFORE UPDATE ON groups
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS groups_updated_at ON groups;
ALTER TABLE groups DROP COLUMN IF EXISTS updated_at;
