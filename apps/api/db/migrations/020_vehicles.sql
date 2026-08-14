-- +goose Up
CREATE TABLE vehicles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  movement_id UUID NOT NULL REFERENCES movements(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  plate_number TEXT NOT NULL, capacity INT NOT NULL CHECK (capacity > 0 AND capacity <= 100),
  driver_name TEXT, driver_phone TEXT,
  status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','departed','arrived','cancelled')),
  departed_at TIMESTAMPTZ, arrived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX vehicles_movement_idx ON vehicles(movement_id);
CREATE INDEX vehicles_operator_idx ON vehicles(operator_id);
CREATE TRIGGER vehicles_set_updated_at BEFORE UPDATE ON vehicles FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose Down
DROP TABLE IF EXISTS vehicles;
