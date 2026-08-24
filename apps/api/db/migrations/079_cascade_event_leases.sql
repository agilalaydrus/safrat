-- +goose Up
-- A database row lock only lasts for the claim statement. A lease keeps a
-- claimed event invisible to other workers while its external side effect is
-- running, and available_at gives failed events bounded exponential backoff.
ALTER TABLE cascade_events
  ADD COLUMN available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN lease_until TIMESTAMPTZ;

DROP INDEX IF EXISTS idx_cascade_events_unprocessed;
CREATE INDEX idx_cascade_events_available
  ON cascade_events (available_at, created_at)
  WHERE processed = FALSE;

-- +goose Down
DROP INDEX IF EXISTS idx_cascade_events_available;
ALTER TABLE cascade_events
  DROP COLUMN IF EXISTS lease_until,
  DROP COLUMN IF EXISTS available_at;
CREATE INDEX idx_cascade_events_unprocessed
  ON cascade_events (created_at)
  WHERE processed = FALSE;
