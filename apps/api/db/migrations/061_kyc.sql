-- +goose Up
-- KYC fields shared by pilgrims and agents. Every Group Leader (Muttawwif)
-- already has a row in `agents` via EnsureAgentForLeader (see
-- internal/repository/agent.go), so extending `agents` covers both Agent
-- and Muttawwif profiles without a third table.
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS nik                  TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS address               TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS kyc_status            TEXT        NOT NULL DEFAULT 'UNVERIFIED'
    CHECK (kyc_status IN ('UNVERIFIED','PENDING_REVIEW','VERIFIED','REJECTED')),
  ADD COLUMN IF NOT EXISTS kyc_source            TEXT        NOT NULL DEFAULT 'ADMIN'
    CHECK (kyc_source IN ('ADMIN','SELF')),
  ADD COLUMN IF NOT EXISTS kyc_verified_by       TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS kyc_verified_at       TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS kyc_rejection_reason  TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS documents_ktp         BOOLEAN     NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS documents_selfie      BOOLEAN     NOT NULL DEFAULT false;

ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','KTP','SELFIE','OTHER'));

ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS nik                   TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS npwp                  TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS address               TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS date_of_birth         DATE,
  ADD COLUMN IF NOT EXISTS passport_number       TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS passport_expiry_date  DATE,
  ADD COLUMN IF NOT EXISTS bank_name             TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS bank_account_number   TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS bank_account_holder   TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS kyc_status            TEXT        NOT NULL DEFAULT 'UNVERIFIED'
    CHECK (kyc_status IN ('UNVERIFIED','PENDING_REVIEW','VERIFIED','REJECTED')),
  ADD COLUMN IF NOT EXISTS kyc_source            TEXT        NOT NULL DEFAULT 'ADMIN'
    CHECK (kyc_source IN ('ADMIN','SELF')),
  ADD COLUMN IF NOT EXISTS kyc_verified_by       TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS kyc_verified_at       TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS kyc_rejection_reason  TEXT        NOT NULL DEFAULT '';

CREATE TABLE agent_documents (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id     UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  operator_id  UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  doc_type     TEXT        NOT NULL CHECK (doc_type IN ('KTP','PASSPORT','SELFIE','NPWP','BANK_BOOK','OTHER')),
  file_url     TEXT        NOT NULL,
  file_name    TEXT        NOT NULL,
  uploaded_by  TEXT        NOT NULL DEFAULT 'operator',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX agent_documents_agent_idx ON agent_documents(agent_id);

-- +goose Down
DROP TABLE IF EXISTS agent_documents;

ALTER TABLE agents
  DROP COLUMN IF EXISTS nik,
  DROP COLUMN IF EXISTS npwp,
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS date_of_birth,
  DROP COLUMN IF EXISTS passport_number,
  DROP COLUMN IF EXISTS passport_expiry_date,
  DROP COLUMN IF EXISTS bank_name,
  DROP COLUMN IF EXISTS bank_account_number,
  DROP COLUMN IF EXISTS bank_account_holder,
  DROP COLUMN IF EXISTS kyc_status,
  DROP COLUMN IF EXISTS kyc_source,
  DROP COLUMN IF EXISTS kyc_verified_by,
  DROP COLUMN IF EXISTS kyc_verified_at,
  DROP COLUMN IF EXISTS kyc_rejection_reason;

ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','OTHER'));

ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS nik,
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS kyc_status,
  DROP COLUMN IF EXISTS kyc_source,
  DROP COLUMN IF EXISTS kyc_verified_by,
  DROP COLUMN IF EXISTS kyc_verified_at,
  DROP COLUMN IF EXISTS kyc_rejection_reason,
  DROP COLUMN IF EXISTS documents_ktp,
  DROP COLUMN IF EXISTS documents_selfie;
