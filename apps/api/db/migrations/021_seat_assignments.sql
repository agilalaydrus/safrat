-- +goose Up
CREATE TABLE seat_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
  pilgrim_id UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id),
  seat_number INT CHECK (seat_number > 0), assigned_by TEXT NOT NULL,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(vehicle_id, pilgrim_id), UNIQUE(vehicle_id, seat_number)
);
CREATE INDEX seat_assignments_vehicle_idx ON seat_assignments(vehicle_id);
CREATE INDEX seat_assignments_pilgrim_idx ON seat_assignments(pilgrim_id);
CREATE INDEX seat_assignments_operator_idx ON seat_assignments(operator_id);
-- +goose Down
DROP TABLE IF EXISTS seat_assignments;
