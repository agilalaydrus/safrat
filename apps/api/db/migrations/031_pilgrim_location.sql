-- +goose Up
-- Periodic (every 5 minutes, driven by the pilgrim PWA in the browser) GPS
-- ping so a pilgrim's last-known position is available the moment an SOS
-- fires, even if the alert itself couldn't get a fresh GPS fix in time
-- (denied permission mid-flight, indoors, etc).
ALTER TABLE pilgrims
  ADD COLUMN last_lat DOUBLE PRECISION,
  ADD COLUMN last_lng DOUBLE PRECISION,
  ADD COLUMN last_location_at TIMESTAMPTZ;

-- A snapshot of where the pilgrim was AT THE MOMENT of the SOS — distinct
-- from pilgrims.last_lat/lng, which keeps moving after the alert is raised.
ALTER TABLE sos_alerts
  ADD COLUMN lat DOUBLE PRECISION,
  ADD COLUMN lng DOUBLE PRECISION;

-- +goose Down
ALTER TABLE sos_alerts DROP COLUMN IF EXISTS lat, DROP COLUMN IF EXISTS lng;
ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS last_lat,
  DROP COLUMN IF EXISTS last_lng,
  DROP COLUMN IF EXISTS last_location_at;
