-- +goose Up
ALTER TABLE cascade_events ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX cascade_events_operator_idempotency_idx
  ON cascade_events (operator_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS cascade_events_operator_idempotency_idx;
ALTER TABLE cascade_events DROP COLUMN IF EXISTS idempotency_key;
