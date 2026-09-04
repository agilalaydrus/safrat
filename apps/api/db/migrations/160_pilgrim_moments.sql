-- +goose Up
-- Momen (§2.14 RENCANA): a photo and a short note from a field officer, sent
-- to a pilgrim's family. Cheap to create, high emotional return — the
-- opposite of FamilyStatus's other fields, which are deliberately withheld
-- (see the CHECK below and family_tracker.proto's comment on FamilyStatus):
-- momen and news are fine to share, raw GPS, room numbers and passport
-- numbers are not.
--
-- Targets either one pilgrim or a whole group (a field officer photographing
-- a bus of forty people should not have to post it forty times) — exactly
-- one of the two, never both and never neither, so a moment always has a
-- knowable audience.
CREATE TABLE pilgrim_moments (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id   UUID        NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  pilgrim_id  UUID        REFERENCES pilgrims(id) ON DELETE CASCADE,
  group_id    UUID        REFERENCES groups(id) ON DELETE CASCADE,
  photo_key   TEXT        NOT NULL,
  caption     TEXT        NOT NULL DEFAULT '',
  created_by  TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK ((pilgrim_id IS NOT NULL) <> (group_id IS NOT NULL))
);
CREATE INDEX pilgrim_moments_pilgrim_idx ON pilgrim_moments (pilgrim_id, created_at DESC) WHERE pilgrim_id IS NOT NULL;
CREATE INDEX pilgrim_moments_group_idx ON pilgrim_moments (group_id, created_at DESC) WHERE group_id IS NOT NULL;
CREATE INDEX pilgrim_moments_season_idx ON pilgrim_moments (operator_id, season_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS pilgrim_moments;
