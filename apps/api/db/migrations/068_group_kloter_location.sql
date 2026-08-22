-- +goose Up
ALTER TABLE groups
  ADD COLUMN kloter_id UUID REFERENCES kloters(id) ON DELETE SET NULL,
  ADD COLUMN current_city TEXT NOT NULL DEFAULT 'INDONESIA',
  ADD COLUMN status TEXT NOT NULL DEFAULT 'ACTIVE',
  ADD COLUMN last_update TIMESTAMPTZ;

ALTER TABLE groups ADD CONSTRAINT groups_city_check
  CHECK (current_city IN ('INDONESIA','MADINAH','MAKKAH','ARAFAH','MUZDALIFAH','MINA','TRANSIT','DEPARTED'));

ALTER TABLE groups ADD CONSTRAINT groups_status_check
  CHECK (status IN ('ACTIVE','IN_IBADAH','EMERGENCY','COMPLETED'));

CREATE INDEX groups_kloter_idx ON groups(kloter_id);

-- Immutable location history — every "Muttawwif updates location" tap
-- appends here, never overwritten, so an operator can reconstruct a
-- group's exact movement trail through the trip.
CREATE TABLE group_location_log (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  group_id    UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  city        TEXT NOT NULL,
  location    TEXT NOT NULL DEFAULT '',
  updated_by  TEXT REFERENCES "user"(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX group_location_log_group_idx ON group_location_log(group_id, created_at DESC);

-- +goose Down
DROP TABLE group_location_log;
DROP INDEX IF EXISTS groups_kloter_idx;
ALTER TABLE groups DROP CONSTRAINT groups_status_check;
ALTER TABLE groups DROP CONSTRAINT groups_city_check;
ALTER TABLE groups DROP COLUMN kloter_id;
ALTER TABLE groups DROP COLUMN current_city;
ALTER TABLE groups DROP COLUMN status;
ALTER TABLE groups DROP COLUMN last_update;
