-- name: EnqueueCascadeEvent :one
INSERT INTO cascade_events (operator_id, event_type, entity_id, payload)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ClaimCascadeEvents :many
-- Atomically claim a batch of unprocessed events (skipping any locked by a
-- concurrent relay) and bump their attempt count. Rows that exhaust
-- max_attempts fall out of the WHERE and become a visible dead-letter
-- backlog rather than looping forever.
UPDATE cascade_events
SET attempts = attempts + 1
WHERE id IN (
  SELECT ce.id FROM cascade_events ce
  WHERE ce.processed = FALSE AND ce.attempts < $1
  ORDER BY ce.created_at
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkCascadeEventProcessed :exec
UPDATE cascade_events
SET processed = TRUE, processed_at = NOW(), last_error = ''
WHERE id = $1;

-- name: MarkCascadeEventFailed :exec
UPDATE cascade_events
SET last_error = $2
WHERE id = $1;
