-- +goose Up
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS payment_status     TEXT        NOT NULL DEFAULT 'UNPAID'
    CHECK (payment_status IN ('UNPAID','DP','PAID')),
  ADD COLUMN IF NOT EXISTS payment_receipt_url TEXT       NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS payment_notes       TEXT       NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS emergency_contact_name  TEXT   NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS emergency_contact_phone TEXT   NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS passport_expiry_date    DATE,
  ADD COLUMN IF NOT EXISTS vaccine_meningitis_date DATE,
  ADD COLUMN IF NOT EXISTS hotel_checked_in        BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS documents_passport      BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS documents_photo         BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS documents_vaccine       BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS payment_status,
  DROP COLUMN IF EXISTS payment_receipt_url,
  DROP COLUMN IF EXISTS payment_notes,
  DROP COLUMN IF EXISTS emergency_contact_name,
  DROP COLUMN IF EXISTS emergency_contact_phone,
  DROP COLUMN IF EXISTS passport_expiry_date,
  DROP COLUMN IF EXISTS vaccine_meningitis_date,
  DROP COLUMN IF EXISTS hotel_checked_in,
  DROP COLUMN IF EXISTS documents_passport,
  DROP COLUMN IF EXISTS documents_photo,
  DROP COLUMN IF EXISTS documents_vaccine;
