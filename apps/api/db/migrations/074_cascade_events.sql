-- +goose Up
-- Transactional outbox for cascade side-effects (Firebase pushes, and later
-- cross-instance dashboard fan-out). A producer inserts a row in the SAME
-- transaction as its authoritative write; the worker relay (cmd/worker,
-- cascade:dispatch) claims unprocessed rows with FOR UPDATE SKIP LOCKED,
-- performs the side-effect, and marks them processed — giving crash-safe,
-- at-least-once, retryable delivery with a durable audit trail.
CREATE TABLE cascade_events (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  operator_id  UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  event_type   TEXT NOT NULL,
  entity_id    UUID,
  payload      JSONB NOT NULL DEFAULT '{}',
  processed    BOOLEAN NOT NULL DEFAULT FALSE,
  attempts     INT NOT NULL DEFAULT 0,
  last_error   TEXT NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ
);

-- Partial index keeps the relay's claim query cheap: it only ever scans the
-- (normally tiny) backlog of unprocessed rows, never the full history.
CREATE INDEX idx_cascade_events_unprocessed
  ON cascade_events (created_at)
  WHERE processed = FALSE;

-- +goose Down
DROP TABLE IF EXISTS cascade_events;
