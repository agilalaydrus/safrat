-- +goose Up
-- End-to-end idempotency for the offline SOS queue: the pilgrim app sends a
-- client-generated key with each alert, so a replay whose original response was
-- lost (after the alert already committed) resolves to the same row instead of
-- raising a duplicate emergency. Empty key = no dedup (partial index skips it).
ALTER TABLE sos_alerts
  ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_sos_alerts_idempotency
  ON sos_alerts (pilgrim_id, idempotency_key)
  WHERE idempotency_key <> '';

-- +goose Down
DROP INDEX IF EXISTS idx_sos_alerts_idempotency;
ALTER TABLE sos_alerts DROP COLUMN IF EXISTS idempotency_key;
