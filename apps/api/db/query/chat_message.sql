-- name: CreateChatMessageFromPilgrim :one
-- See CreateChatMessageFromUser — same replay protection, scoped to the pilgrim.
INSERT INTO chat_messages (operator_id, group_id, sender_pilgrim_id, body, idempotency_key)
SELECT sqlc.arg(operator_id), p.group_id, p.id, sqlc.arg(body), sqlc.arg(idempotency_key)
FROM pilgrims p
WHERE p.id = sqlc.arg(sender_pilgrim_id)
  AND p.operator_id = sqlc.arg(operator_id)
  AND p.group_id = sqlc.arg(group_id)
ON CONFLICT (sender_pilgrim_id, idempotency_key) WHERE idempotency_key <> '' AND sender_pilgrim_id IS NOT NULL
DO UPDATE SET body = chat_messages.body
RETURNING *;

-- name: CreateChatMessageFromUser :one
-- The conflict target is the partial idempotency index, so a replayed send
-- resolves to the row it already created instead of posting a second copy.
-- DO UPDATE rather than DO NOTHING: the caller needs the row back to echo it
-- into the thread, and DO NOTHING returns none.
INSERT INTO chat_messages (operator_id, group_id, sender_user_id, body, idempotency_key)
SELECT sqlc.arg(operator_id), g.id, sqlc.arg(sender_user_id), sqlc.arg(body), sqlc.arg(idempotency_key)
FROM groups g
WHERE g.id = sqlc.arg(group_id) AND g.operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR NOT EXISTS (
    SELECT 1 FROM pilgrims member
    WHERE member.group_id = g.id AND member.is_substituted = false
      AND member.branch_id IS DISTINCT FROM sqlc.narg(branch_scope)::uuid
  ))
ON CONFLICT (sender_user_id, idempotency_key) WHERE idempotency_key <> '' AND sender_user_id IS NOT NULL
DO UPDATE SET body = chat_messages.body
RETURNING *;

-- name: ListChatMessagesByGroup :many
SELECT m.*, p.full_name AS sender_pilgrim_name, u.name AS sender_user_name
FROM chat_messages m
LEFT JOIN pilgrims p ON p.id = m.sender_pilgrim_id
LEFT JOIN "user" u ON u.id = m.sender_user_id
WHERE m.group_id = $1 AND m.operator_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR NOT EXISTS (
    SELECT 1 FROM pilgrims member
    WHERE member.group_id = m.group_id AND member.is_substituted = false
      AND member.branch_id IS DISTINCT FROM sqlc.narg(branch_scope)::uuid
  ))
ORDER BY m.created_at ASC
LIMIT 200;
