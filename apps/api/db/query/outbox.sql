-- name: EnqueueCascadeEvent :one
INSERT INTO cascade_events (operator_id, event_type, entity_id, payload)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ClaimCascadeEvents :many
-- Atomically lease a batch of due events. The lease survives this statement's
-- row lock, preventing another worker from processing the same event while an
-- external push/data cascade is still in flight.
UPDATE cascade_events
SET attempts = attempts + 1,
    lease_until = NOW() + INTERVAL '30 seconds'
WHERE id IN (
  SELECT ce.id FROM cascade_events ce
  WHERE ce.processed = FALSE
    AND ce.attempts < $1
    AND ce.available_at <= NOW()
    AND (ce.lease_until IS NULL OR ce.lease_until <= NOW())
  ORDER BY ce.created_at
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkCascadeEventProcessed :exec
UPDATE cascade_events
SET processed = TRUE, processed_at = NOW(), last_error = '', lease_until = NULL
WHERE id = $1;

-- name: MarkCascadeEventFailed :exec
UPDATE cascade_events
SET last_error = $2,
    lease_until = NULL,
    available_at = NOW() + power(2, LEAST(attempts, 5)) * INTERVAL '1 second'
WHERE id = $1;
