-- name: UpsertPushSubscription :one
INSERT INTO push_subscriptions (operator_id, user_id, fcm_token)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, fcm_token) DO UPDATE SET fcm_token = EXCLUDED.fcm_token
RETURNING *;

-- name: ListPushTokensForOperator :many
SELECT DISTINCT fcm_token FROM push_subscriptions
WHERE operator_id = $1;

-- name: UpsertPilgrimPushToken :exec
INSERT INTO pilgrim_push_tokens (operator_id, pilgrim_id, fcm_token)
VALUES ($1, $2, $3)
ON CONFLICT (pilgrim_id, fcm_token) DO NOTHING;

-- name: ListPushTokensForGroup :many
SELECT DISTINCT t.fcm_token
FROM pilgrim_push_tokens t
JOIN pilgrims p ON p.id = t.pilgrim_id
WHERE t.operator_id = $1 AND p.group_id = $2 AND p.is_substituted = false;

-- name: ListPushTokensForKloter :many
SELECT DISTINCT t.fcm_token
FROM pilgrim_push_tokens t
JOIN pilgrims p ON p.id = t.pilgrim_id
WHERE t.operator_id = $1 AND p.kloter_id = $2 AND p.is_substituted = false;

-- name: DeletePushSubscriptionToken :exec
DELETE FROM push_subscriptions
WHERE operator_id = $1 AND fcm_token = $2;

-- name: DeletePilgrimPushToken :exec
DELETE FROM pilgrim_push_tokens
WHERE operator_id = $1 AND fcm_token = $2;
