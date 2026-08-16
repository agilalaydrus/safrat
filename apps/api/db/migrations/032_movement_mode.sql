-- +goose Up
-- Movements aren't only ground bus transfers — Hajj/Umrah itineraries also
-- include flights and, increasingly, high-speed rail (Haramain). The vehicle
-- fields underneath (plate_number/driver_name/driver_phone) stay generic
-- strings; only the label shown to the operator changes based on mode.
ALTER TABLE movements
  ADD COLUMN mode TEXT NOT NULL DEFAULT 'BUS' CHECK (mode IN ('BUS','FLIGHT','TRAIN'));

-- +goose Down
ALTER TABLE movements DROP COLUMN mode;
