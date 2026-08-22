-- +goose Up
-- Rounds out pilgrim KYC to the fields Saudi visa/manifest submission
-- actually asks for (father's name, place of birth, marital status,
-- occupation), and the pilgrim_documents checklist to the full PPIU/Saudi
-- entry document set (KTP was already added in 061; KK and a mahram-proof
-- document — akta nikah/kelahiran — were still missing).
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS place_of_birth  TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS marital_status  TEXT NOT NULL DEFAULT ''
    CHECK (marital_status IN ('', 'SINGLE', 'MARRIED', 'DIVORCED', 'WIDOWED')),
  ADD COLUMN IF NOT EXISTS occupation      TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS father_name     TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS documents_kk            BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS documents_mahram_proof  BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS insurance_start_date        DATE,
  ADD COLUMN IF NOT EXISTS insurance_end_date          DATE,
  ADD COLUMN IF NOT EXISTS insurance_beneficiary_name     TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS insurance_beneficiary_relation TEXT NOT NULL DEFAULT '';

ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','KTP','SELFIE','KK','MAHRAM_PROOF','OTHER'));

-- +goose Down
ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','KTP','SELFIE','OTHER'));

ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS place_of_birth,
  DROP COLUMN IF EXISTS marital_status,
  DROP COLUMN IF EXISTS occupation,
  DROP COLUMN IF EXISTS father_name,
  DROP COLUMN IF EXISTS documents_kk,
  DROP COLUMN IF EXISTS documents_mahram_proof,
  DROP COLUMN IF EXISTS insurance_start_date,
  DROP COLUMN IF EXISTS insurance_end_date,
  DROP COLUMN IF EXISTS insurance_beneficiary_name,
  DROP COLUMN IF EXISTS insurance_beneficiary_relation;
