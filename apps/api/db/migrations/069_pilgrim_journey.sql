-- +goose Up
-- Current status per pilgrim (one row, upserted) plus an immutable log of
-- every transition — the log is the audit trail, the status row is just
-- the fast "where is this pilgrim right now" read.
CREATE TABLE pilgrim_journey_status (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id   UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  status       TEXT NOT NULL DEFAULT 'REGISTERED',
  updated_by   TEXT REFERENCES "user"(id),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  notes        TEXT NOT NULL DEFAULT '',
  UNIQUE(pilgrim_id)
);

CREATE TABLE pilgrim_journey_log (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id  UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  from_status TEXT NOT NULL,
  to_status   TEXT NOT NULL,
  updated_by  TEXT REFERENCES "user"(id),
  notes       TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX pilgrim_journey_log_pilgrim_idx ON pilgrim_journey_log(pilgrim_id, created_at DESC);

INSERT INTO pilgrim_journey_status (operator_id, pilgrim_id, status)
SELECT operator_id, id, 'REGISTERED' FROM pilgrims WHERE is_substituted = false
ON CONFLICT (pilgrim_id) DO NOTHING;

-- +goose Down
DROP TABLE pilgrim_journey_log;
DROP TABLE pilgrim_journey_status;
