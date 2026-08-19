-- +goose Up
ALTER TABLE pilgrims
  ADD COLUMN substitution_reason TEXT NOT NULL DEFAULT '',
  ADD COLUMN substituted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS substitution_reason,
  DROP COLUMN IF EXISTS substituted_at;
