-- name: CreateChatMessageFromPilgrim :one
INSERT INTO chat_messages (operator_id, group_id, sender_pilgrim_id, body)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateChatMessageFromUser :one
INSERT INTO chat_messages (operator_id, group_id, sender_user_id, body)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListChatMessagesByGroup :many
SELECT m.*, p.full_name AS sender_pilgrim_name, u.name AS sender_user_name
FROM chat_messages m
LEFT JOIN pilgrims p ON p.id = m.sender_pilgrim_id
LEFT JOIN "user" u ON u.id = m.sender_user_id
WHERE m.group_id = $1 AND m.operator_id = $2
ORDER BY m.created_at ASC
LIMIT 200;
