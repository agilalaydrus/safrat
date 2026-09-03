-- +goose Up
-- Where a registration came from, kept on the registration itself.
--
-- The funnel already records utm_source against a visit, but a visit and a
-- registration cannot be tied together across days: the visitor token is
-- deliberately reset every midnight so nobody can be followed. Umrah is a
-- decision people take weeks over, so attribution held only in the funnel would
-- credit whichever channel happened to bring them back on the day they finally
-- signed up — and under-report every channel that starts a long consideration.
--
-- Keeping it on the row captures the channel present on the visit where the
-- form was actually completed. Not perfect, honest, and it needs no cookie.
ALTER TABLE pilgrim_registrations
  ADD COLUMN utm_source   TEXT NOT NULL DEFAULT '' CHECK (length(utm_source) <= 80),
  ADD COLUMN utm_campaign TEXT NOT NULL DEFAULT '' CHECK (length(utm_campaign) <= 120);

-- Partial: most rows carry no campaign, and indexing empty strings helps
-- nothing.
CREATE INDEX pilgrim_registrations_utm_idx
  ON pilgrim_registrations (operator_id, utm_source)
  WHERE utm_source <> '';

-- +goose Down
DROP INDEX IF EXISTS pilgrim_registrations_utm_idx;
ALTER TABLE pilgrim_registrations DROP COLUMN IF EXISTS utm_campaign;
ALTER TABLE pilgrim_registrations DROP COLUMN IF EXISTS utm_source;
