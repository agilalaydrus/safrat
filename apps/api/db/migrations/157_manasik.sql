-- +goose Up
-- Manasik: ritual-practice training, distinct from a Product's sales-facing
-- itinerary and from Rundown's operational day plan for a kloter already in
-- transit. Season-scoped like Checklist templates — each season runs its own
-- manasik schedule.
CREATE TABLE manasik_curricula (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id   UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  title       TEXT        NOT NULL,
  description TEXT        NOT NULL DEFAULT '',
  sort_order  INT         NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX manasik_curricula_season_idx ON manasik_curricula(operator_id, season_id, sort_order);

-- A session covers one curriculum topic (or none, for a general/makeup
-- session) and is optionally scoped to a single kloter — most manasik run
-- for a whole season's intake before kloters are even finalized.
CREATE TABLE manasik_sessions (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id       UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  curriculum_id   UUID        REFERENCES manasik_curricula(id) ON DELETE SET NULL,
  kloter_id       UUID        REFERENCES kloters(id) ON DELETE SET NULL,
  title           TEXT        NOT NULL,
  location        TEXT        NOT NULL DEFAULT '',
  instructor_name TEXT        NOT NULL DEFAULT '',
  scheduled_at    TIMESTAMPTZ NOT NULL,
  duration_minutes INT        NOT NULL DEFAULT 60 CHECK (duration_minutes > 0),
  capacity        INT         NOT NULL DEFAULT 0 CHECK (capacity >= 0),
  notes           TEXT        NOT NULL DEFAULT '',
  status          TEXT        NOT NULL DEFAULT 'SCHEDULED'
                  CHECK (status IN ('SCHEDULED','COMPLETED','CANCELLED')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX manasik_sessions_season_idx ON manasik_sessions(operator_id, season_id, scheduled_at);
CREATE INDEX manasik_sessions_kloter_idx ON manasik_sessions(kloter_id);
CREATE TRIGGER manasik_sessions_set_updated_at BEFORE UPDATE ON manasik_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Attendance is taken as one roll-call per session, not row by row — so it's
-- upserted as a whole set (see RecordAttendance), and a pilgrim appears at
-- most once per session by construction.
CREATE TABLE manasik_attendance (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  session_id   UUID        NOT NULL REFERENCES manasik_sessions(id) ON DELETE CASCADE,
  pilgrim_id   UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  status       TEXT        NOT NULL CHECK (status IN ('PRESENT','ABSENT','EXCUSED')),
  notes        TEXT        NOT NULL DEFAULT '',
  recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (session_id, pilgrim_id)
);
CREATE INDEX manasik_attendance_pilgrim_idx ON manasik_attendance(pilgrim_id);

-- +goose Down
DROP TABLE manasik_attendance;
DROP TABLE manasik_sessions;
DROP TABLE manasik_curricula;
