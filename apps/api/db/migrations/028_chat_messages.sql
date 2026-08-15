-- +goose Up
CREATE TABLE chat_messages (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id       UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  group_id          UUID        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  sender_pilgrim_id UUID        REFERENCES pilgrims(id) ON DELETE SET NULL,
  sender_user_id    TEXT        REFERENCES "user"(id) ON DELETE SET NULL,
  body              TEXT        NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (num_nonnulls(sender_pilgrim_id, sender_user_id) = 1)
);
CREATE INDEX chat_messages_group_idx ON chat_messages(group_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS chat_messages;
