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
