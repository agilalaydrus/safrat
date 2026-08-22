-- +goose Up
-- A kloter's single kloters.flight_number was always a lie for anything
-- beyond the simplest itinerary — transit legs and the return flight are
-- real, separately-scheduled events, often on a different airline
-- entirely. movements already models "one leg, optionally scoped to one
-- kloter" (see 034_movement_kloter.sql), so per-leg flight detail belongs
-- there, not as a second flat field on kloters.
ALTER TABLE movements
  ADD COLUMN IF NOT EXISTS airline        TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS flight_number  TEXT NOT NULL DEFAULT '',
  -- '' covers movements that aren't part of the international trip at all
  -- (a local Makkah<->Madinah shuttle) — only DEPARTURE/RETURN legs get
  -- grouped into the "keberangkatan"/"kepulangan" itinerary view. More
  -- than one movement tagged the same direction, ordered by scheduled_at,
  -- is how a transit shows up — no separate TRANSIT value needed.
  ADD COLUMN IF NOT EXISTS trip_leg       TEXT NOT NULL DEFAULT ''
    CHECK (trip_leg IN ('', 'DEPARTURE', 'RETURN'));

-- +goose Down
ALTER TABLE movements
  DROP COLUMN IF EXISTS airline,
  DROP COLUMN IF EXISTS flight_number,
  DROP COLUMN IF EXISTS trip_leg;
