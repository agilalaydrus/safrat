-- +goose Up
CREATE TABLE lost_reports (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id  UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  group_id    UUID        REFERENCES groups(id) ON DELETE SET NULL,
  latitude    FLOAT8      NOT NULL,
  longitude   FLOAT8      NOT NULL,
  last_known_location TEXT NOT NULL DEFAULT '',
  status      TEXT        NOT NULL DEFAULT 'LOST'
              CHECK (status IN ('LOST','FOUND','RESOLVED')),
  resolved_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX lost_reports_group_idx    ON lost_reports(group_id, status);
CREATE INDEX lost_reports_operator_idx ON lost_reports(operator_id, status);

-- +goose Down
DROP TABLE IF EXISTS lost_reports;
