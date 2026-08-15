-- +goose Up
CREATE TABLE push_subscriptions (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  user_id     TEXT        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  fcm_token   TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, fcm_token)
);
CREATE INDEX push_subscriptions_operator_idx ON push_subscriptions(operator_id);

-- +goose Down
DROP TABLE IF EXISTS push_subscriptions;
