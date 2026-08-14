-- +goose Up
CREATE TABLE rooms (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hotel_id UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  room_number TEXT NOT NULL,
  room_type TEXT NOT NULL,
  capacity INT NOT NULL CHECK (capacity > 0),
  floor INT,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(hotel_id, room_number)
);
CREATE INDEX rooms_hotel_idx ON rooms(hotel_id);
CREATE INDEX rooms_operator_idx ON rooms(operator_id);
-- +goose Down
DROP TABLE IF EXISTS rooms;
