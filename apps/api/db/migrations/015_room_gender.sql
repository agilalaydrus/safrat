-- +goose Up
ALTER TABLE rooms
  ADD COLUMN gender TEXT NOT NULL DEFAULT 'male'
  CHECK (gender IN ('male', 'female', 'family'));

-- +goose Down
ALTER TABLE rooms DROP COLUMN gender;
