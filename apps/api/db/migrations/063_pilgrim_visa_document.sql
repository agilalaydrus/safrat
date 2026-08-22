-- +goose Up
-- Visa (Umrah/Hajj visa issued via Nusuk/eHajj) was missing from the PPIU
-- + Saudi entry document checklist added in 062 — it's a separate document
-- from the passport itself, not implied by it.
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS documents_visa BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS visa_number     TEXT    NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS visa_expiry_date DATE;

ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','KTP','SELFIE','KK','MAHRAM_PROOF','VISA','OTHER'));

-- +goose Down
ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','KTP','SELFIE','KK','MAHRAM_PROOF','OTHER'));

ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS documents_visa,
  DROP COLUMN IF EXISTS visa_number,
  DROP COLUMN IF EXISTS visa_expiry_date;
