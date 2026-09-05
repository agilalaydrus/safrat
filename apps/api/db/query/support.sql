-- name: CreateSupportTicket :one
INSERT INTO support_tickets (operator_id, subject, priority, created_by_user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListSupportTickets :many
SELECT * FROM support_tickets WHERE operator_id = $1 ORDER BY created_at DESC;

-- name: GetSupportTicket :one
SELECT * FROM support_tickets WHERE id = $1 AND operator_id = $2;

-- name: CreateSupportTicketMessage :one
INSERT INTO support_ticket_messages (ticket_id, body, author_user_id, author_name, author_is_platform)
VALUES ($1, $2, $3, $4, false)
RETURNING *;

-- name: ListSupportTicketMessages :many
SELECT * FROM support_ticket_messages WHERE ticket_id = $1 ORDER BY created_at;

-- name: CloseSupportTicket :one
UPDATE support_tickets SET status = 'CLOSED', resolved_at = NOW()
WHERE id = $1 AND operator_id = $2 AND status NOT IN ('RESOLVED','CLOSED')
RETURNING *;

-- Platform-side (C5, TUGAS-PANEL-SAAS.md): unscoped by operator on purpose —
-- this is the admin inbox across every tenant.

-- name: ListAllSupportTickets :many
SELECT t.*, o.name AS operator_name
FROM support_tickets t
JOIN operators o ON o.id = t.operator_id
ORDER BY t.created_at DESC;

-- name: GetSupportTicketAsPlatform :one
SELECT t.*, o.name AS operator_name
FROM support_tickets t
JOIN operators o ON o.id = t.operator_id
WHERE t.id = $1;

-- name: CreateSupportTicketMessageAsPlatform :one
INSERT INTO support_ticket_messages (ticket_id, body, author_user_id, author_name, author_is_platform)
VALUES ($1, $2, $3, $4, true)
RETURNING *;

-- SetSupportTicketStatus can neither touch an already-CLOSED ticket nor set
-- one to CLOSED — both guarded in the WHERE clause, not just the target
-- value, because a query that only rejected an already-closed row would
-- still happily set an OPEN one straight to CLOSED. CLOSED is exclusively
-- CloseSupportTicket's, the operator's own action.
-- resolved_at is set or cleared depending on the direction: this is not
-- forward-only, so moving a ticket back from RESOLVED to IN_PROGRESS must
-- also clear it, or the CHECK constraint tying the two together would reject
-- the update.
-- name: SetSupportTicketStatus :one
UPDATE support_tickets
SET status = $2, resolved_at = CASE WHEN $2 = 'RESOLVED' THEN NOW() ELSE NULL END
WHERE id = $1 AND status != 'CLOSED' AND $2 != 'CLOSED'
RETURNING *;
