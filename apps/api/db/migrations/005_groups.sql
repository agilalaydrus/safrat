-- +goose Up
CREATE TABLE groups (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  season_id   UUID NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  leader_id   UUID REFERENCES users(id) ON DELETE SET NULL,
  name        TEXT NOT NULL,
  capacity    INT NOT NULL DEFAULT 40 CHECK (capacity > 0),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS groups;
