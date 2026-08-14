-- +goose Up
ALTER TABLE room_allocations
  ADD COLUMN assigned_by TEXT NOT NULL DEFAULT 'system';

-- +goose Down
ALTER TABLE room_allocations DROP COLUMN assigned_by;
