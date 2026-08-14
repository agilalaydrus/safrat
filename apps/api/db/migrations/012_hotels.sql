-- +goose Up
CREATE TABLE hotels (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id UUID NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  city TEXT NOT NULL,
  star_rating INT,
  address TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX hotels_operator_season_idx ON hotels(operator_id, season_id);
-- +goose Down
DROP TABLE IF EXISTS hotels;
