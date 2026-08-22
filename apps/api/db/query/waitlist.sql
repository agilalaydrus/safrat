-- name: CountSeasonPilgrims :one
SELECT COUNT(*) FROM pilgrims
WHERE season_id = $1 AND operator_id = $2 AND NOT is_substituted AND status = 'ACTIVE';

-- name: GetSeasonCapacity :one
SELECT capacity FROM seasons
WHERE id = $1 AND operator_id = $2;

-- name: JoinWaitlist :one
INSERT INTO season_waitlists (operator_id, season_id, full_name, email, phone, product_id, position)
VALUES (
  $1, $2, $3, $4, $5, $6,
  COALESCE((SELECT MAX(position) FROM season_waitlists WHERE season_id = $2 AND status = 'WAITING'), 0) + 1
)
RETURNING *;

-- name: GetWaitlistEntryByEmail :one
SELECT * FROM season_waitlists
WHERE season_id = $1 AND email = $2
LIMIT 1;

-- name: ListWaitlist :many
SELECT * FROM season_waitlists
WHERE operator_id = $1 AND season_id = $2
ORDER BY position ASC;

-- name: GetNextWaiting :one
SELECT * FROM season_waitlists
WHERE season_id = $1 AND status = 'WAITING'
ORDER BY position ASC
LIMIT 1;

-- name: PromoteWaitlistEntry :one
UPDATE season_waitlists
SET status = 'PROMOTED', promoted_at = NOW(), expires_at = NOW() + INTERVAL '48 hours'
WHERE id = $1 AND operator_id = $2 AND status = 'WAITING'
RETURNING *;

-- name: ConfirmWaitlistEntry :one
UPDATE season_waitlists
SET status = 'CONFIRMED'
WHERE id = $1 AND season_id = $2 AND email = $3 AND status = 'PROMOTED' AND expires_at > NOW()
RETURNING *;

-- name: AdminConfirmWaitlistEntry :one
-- Staff-facing confirm — no email match, no expiry check (a trusted staff
-- action after calling/WhatsApping the person directly, not an unattended
-- public link). Works from WAITING or PROMOTED.
UPDATE season_waitlists
SET status = 'CONFIRMED'
WHERE id = $1 AND operator_id = $2 AND status IN ('WAITING', 'PROMOTED')
RETURNING *;

-- name: ExpirePromotedEntries :many
-- Bulk sweep for the worker — flips every stale PROMOTED entry to EXPIRED
-- and returns the affected season_ids so the caller can promote the next
-- person in line for each.
UPDATE season_waitlists
SET status = 'EXPIRED'
WHERE status = 'PROMOTED' AND expires_at < NOW()
RETURNING season_id, operator_id;

-- name: RemoveFromWaitlist :exec
UPDATE season_waitlists
SET status = 'REMOVED'
WHERE id = $1 AND operator_id = $2;

-- name: LeaveWaitlist :exec
-- Public — authenticated by email match only (no operator session).
UPDATE season_waitlists
SET status = 'REMOVED'
WHERE season_id = $1 AND email = $2 AND status IN ('WAITING','PROMOTED');
