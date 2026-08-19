-- +goose Up

-- Maximum pilgrims the operator will accept for this season. 0 means
-- unlimited (no capacity gate — JoinWaitlist always redirects to
-- registration instead of queuing when this is 0).
ALTER TABLE seasons
  ADD COLUMN capacity INTEGER NOT NULL DEFAULT 0;

-- Waiting list entry for a prospective jamaah joining once a season is
-- full. One entry per email per season — DB-enforced, last line of
-- defence behind the application-level duplicate check.
CREATE TABLE season_waitlists (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id)   ON DELETE CASCADE,
  full_name     TEXT        NOT NULL,
  email         TEXT        NOT NULL,
  phone         TEXT        NOT NULL DEFAULT '',
  product_id    UUID        REFERENCES products(id) ON DELETE SET NULL,
  position      INTEGER     NOT NULL,
  status        TEXT        NOT NULL DEFAULT 'WAITING'
                            CHECK (status IN ('WAITING','PROMOTED','CONFIRMED','EXPIRED','REMOVED')),
  promoted_at   TIMESTAMPTZ,
  expires_at    TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, email)
);

CREATE INDEX season_waitlists_operator_season_idx ON season_waitlists(operator_id, season_id, status);
CREATE INDEX season_waitlists_position_idx        ON season_waitlists(season_id, position);

CREATE TRIGGER season_waitlists_set_updated_at
  BEFORE UPDATE ON season_waitlists
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS season_waitlists;
ALTER TABLE seasons DROP COLUMN IF EXISTS capacity;
