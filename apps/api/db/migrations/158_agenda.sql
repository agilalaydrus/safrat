-- +goose Up
-- Agenda: the combined activity calendar (§2.10 RENCANA-DASHBOARD-TRAVEL).
-- Distinct from staff_schedule's `/schedule` (staff task assignments) and
-- `/my-schedule` (one staff member's own tasks) — this is jamaah-facing
-- operational activity: manasik sessions, kloter departures/returns, and
-- internal events, read together on one timeline.
--
-- Only internal events get a table here. Manasik sessions and kloter
-- departures/returns already live in manasik_sessions and
-- kloter_itinerary_segments (T4.2 and T3.3) — duplicating them into this
-- table would let the two copies drift, so ListAgenda reads all three
-- sources live instead.
CREATE TABLE agenda_events (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- NULL means head office ("agenda pusat"). Kloter and manasik activity is
  -- never branch-owned — a branch does not run its own kloter — so the
  -- pusat/cabang split in the design only applies to this table; the other
  -- two sources always show regardless of which branch is selected.
  branch_id   UUID        REFERENCES branches(id) ON DELETE CASCADE,
  -- Optional: most internal events (a head-office meeting, a policy
  -- briefing) are not tied to one season's departure cycle. NULL events show
  -- on every season's agenda instead of disappearing between seasons.
  season_id   UUID        REFERENCES seasons(id) ON DELETE CASCADE,
  title       TEXT        NOT NULL CHECK (length(trim(title)) > 0),
  location    TEXT        NOT NULL DEFAULT '',
  starts_at   TIMESTAMPTZ NOT NULL,
  ends_at     TIMESTAMPTZ,
  notes       TEXT        NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (ends_at IS NULL OR ends_at >= starts_at)
);
CREATE INDEX agenda_events_operator_starts_idx ON agenda_events(operator_id, starts_at);
CREATE INDEX agenda_events_branch_idx ON agenda_events(branch_id) WHERE branch_id IS NOT NULL;
CREATE INDEX agenda_events_season_idx ON agenda_events(season_id) WHERE season_id IS NOT NULL;
CREATE TRIGGER agenda_events_set_updated_at BEFORE UPDATE ON agenda_events FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Reuses the trigger function migration 128 defined for pilgrims/orders/etc —
-- a stray branch id from another tenant must not silently scope this table
-- to nothing.
CREATE TRIGGER agenda_events_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON agenda_events
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();

-- +goose Down
DROP TRIGGER IF EXISTS agenda_events_branch_matches_operator ON agenda_events;
DROP TABLE IF EXISTS agenda_events;
