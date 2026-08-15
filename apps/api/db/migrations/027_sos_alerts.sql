-- +goose Up
CREATE TABLE sos_alerts (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id      UUID        NOT NULL REFERENCES pilgrims(id),
  status          TEXT        NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','ACKNOWLEDGED','ESCALATED','RESOLVED')),
  acknowledged_by TEXT        REFERENCES "user"(id) ON DELETE SET NULL,
  acknowledged_at TIMESTAMPTZ,
  resolved_by     TEXT        REFERENCES "user"(id) ON DELETE SET NULL,
  resolved_at     TIMESTAMPTZ,
  notes           TEXT        NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX sos_alerts_operator_status_idx ON sos_alerts(operator_id, status);
CREATE INDEX sos_alerts_pilgrim_idx ON sos_alerts(pilgrim_id);

-- +goose Down
DROP TABLE IF EXISTS sos_alerts;
