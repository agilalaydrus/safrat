-- +goose Up
-- logo_url already exists on operators (nullable TEXT) — do not re-add it here.
ALTER TABLE operators
  ADD COLUMN IF NOT EXISTS description         TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS whatsapp_number     TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS website             TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS address             TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS city                TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS is_profile_complete BOOLEAN NOT NULL DEFAULT FALSE;
  -- is_profile_complete flips TRUE once the operator finishes the onboarding
  -- wizard (UpdateOperatorProfile sets it).

-- +goose Down
ALTER TABLE operators
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS whatsapp_number,
  DROP COLUMN IF EXISTS website,
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS city,
  DROP COLUMN IF EXISTS is_profile_complete;
