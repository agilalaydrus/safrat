-- +goose Up
CREATE TABLE ritual_templates (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_type TEXT NOT NULL, -- HAJJ | UMRAH
  name        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  order_num   INT NOT NULL DEFAULT 0,
  is_required BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ritual_templates_operator_idx ON ritual_templates(operator_id, season_type, order_num);

CREATE TABLE pilgrim_rituals (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id   UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  ritual_id    UUID NOT NULL REFERENCES ritual_templates(id) ON DELETE CASCADE,
  completed    BOOLEAN NOT NULL DEFAULT FALSE,
  completed_at TIMESTAMPTZ,
  completed_by TEXT REFERENCES "user"(id),
  notes        TEXT NOT NULL DEFAULT '',
  UNIQUE(pilgrim_id, ritual_id)
);
CREATE INDEX pilgrim_rituals_pilgrim_idx ON pilgrim_rituals(pilgrim_id);

CREATE TYPE health_severity AS ENUM ('RINGAN', 'SEDANG', 'BERAT');

CREATE TABLE pilgrim_health_reports (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id   UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  group_id     UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  reported_by  TEXT REFERENCES "user"(id),
  severity     health_severity NOT NULL DEFAULT 'RINGAN',
  symptoms     TEXT NOT NULL,
  action_taken TEXT NOT NULL DEFAULT '',
  resolved     BOOLEAN NOT NULL DEFAULT FALSE,
  resolved_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX pilgrim_health_reports_operator_idx ON pilgrim_health_reports(operator_id, resolved, created_at DESC);
CREATE INDEX pilgrim_health_reports_group_idx ON pilgrim_health_reports(group_id);

-- pilgrim_push_tokens is deliberately separate from push_subscriptions
-- (which is keyed to a Better Auth user_id, i.e. staff/leader devices) —
-- the pilgrim PWA is mostly public (app_access_code), no Better Auth
-- session guaranteed, so it registers here instead, keyed to pilgrim_id
-- directly.
CREATE TABLE pilgrim_push_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id  UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  fcm_token   TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(pilgrim_id, fcm_token)
);
CREATE INDEX pilgrim_push_tokens_pilgrim_idx ON pilgrim_push_tokens(pilgrim_id);

-- +goose Down
DROP TABLE pilgrim_push_tokens;
DROP TABLE pilgrim_health_reports;
DROP TYPE health_severity;
DROP TABLE pilgrim_rituals;
DROP TABLE ritual_templates;
