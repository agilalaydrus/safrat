-- name: UpsertPushSubscription :one
INSERT INTO push_subscriptions (operator_id, user_id, fcm_token)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, fcm_token) DO UPDATE SET fcm_token = EXCLUDED.fcm_token
RETURNING *;

-- name: ListPushTokensForOperator :many
SELECT DISTINCT fcm_token FROM push_subscriptions
WHERE operator_id = $1;
