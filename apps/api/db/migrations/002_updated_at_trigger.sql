-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER operators_set_updated_at
BEFORE UPDATE ON operators
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS operators_set_updated_at ON operators;
DROP FUNCTION IF EXISTS set_updated_at();
