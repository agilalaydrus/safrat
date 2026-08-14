-- +goose Up
CREATE TABLE room_allocations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  pilgrim_id UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  allocated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(pilgrim_id)
);
CREATE INDEX room_allocations_room_idx ON room_allocations(room_id);
CREATE INDEX room_allocations_operator_idx ON room_allocations(operator_id);
-- +goose Down
DROP TABLE IF EXISTS room_allocations;
