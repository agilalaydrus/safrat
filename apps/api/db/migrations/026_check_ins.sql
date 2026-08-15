-- +goose Up
CREATE TABLE check_ins (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id    UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  movement_id    UUID        NOT NULL REFERENCES movements(id) ON DELETE CASCADE,
  pilgrim_id     UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  type           TEXT        NOT NULL CHECK (type IN ('DEPARTURE','ARRIVAL')),
  checked_in_by  TEXT        REFERENCES "user"(id) ON DELETE SET NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (movement_id, pilgrim_id, type)
);
CREATE INDEX check_ins_movement_idx ON check_ins(movement_id);
CREATE INDEX check_ins_pilgrim_idx ON check_ins(pilgrim_id);

-- +goose Down
DROP TABLE IF EXISTS check_ins;
