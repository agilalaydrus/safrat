-- +goose Up

-- KYC lived as columns scattered across agents and pilgrims: an identity was a
-- property of whichever role record happened to hold it, with no way to ask
-- "whose identity is this" or to hand one to a regulator on request.
--
-- It is its own record now, with the account it belongs to named explicitly.
-- That is what makes it retrievable: one place to look, one relation to follow,
-- rather than a join per role guessing where the person turned up.
CREATE TABLE kyc_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- The Better Auth account, when the person has one. Nullable because a
  -- jamaah can be registered by staff and never sign in — their identity still
  -- has to be verifiable.
  user_id TEXT,
  -- Which role record this identity was collected against.
  subject_type TEXT NOT NULL CHECK (subject_type IN ('AGENT', 'PILGRIM')),
  subject_id UUID NOT NULL,
  -- Kept alongside so a record is legible without joining back out to a role
  -- table that may since have been renamed or deactivated.
  full_name TEXT NOT NULL DEFAULT '',

  -- Encrypted at rest, by the application, before they ever reach a row.
  -- AES-256-GCM with a random nonce each time, so the same identity number
  -- never produces the same ciphertext and equal values reveal nothing.
  -- Searching by these is therefore impossible, which is acceptable: nothing
  -- searches by them, and a searchable identity number is a leak waiting for a
  -- query.
  nik_encrypted TEXT NOT NULL DEFAULT '',
  npwp_encrypted TEXT NOT NULL DEFAULT '',

  address TEXT NOT NULL DEFAULT '',
  place_of_birth TEXT NOT NULL DEFAULT '',
  date_of_birth DATE,

  status TEXT NOT NULL DEFAULT 'PENDING_REVIEW'
    CHECK (status IN ('PENDING_REVIEW', 'VERIFIED', 'REJECTED')),
  -- SELF when the person submitted it, STAFF when an operator entered it.
  source TEXT NOT NULL DEFAULT 'SELF',
  verified_by TEXT NOT NULL DEFAULT '',
  verified_at TIMESTAMPTZ,
  rejection_reason TEXT NOT NULL DEFAULT '',

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One identity record per role record. A second would make "which one is
-- current" unanswerable at exactly the moment somebody is asking.
CREATE UNIQUE INDEX kyc_records_subject_idx ON kyc_records (subject_type, subject_id);
-- Looking up by account is the point of the user_id column.
CREATE INDEX kyc_records_user_idx ON kyc_records (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX kyc_records_operator_status_idx ON kyc_records (operator_id, status, created_at DESC);

-- The legacy columns on agents and pilgrims stay for now, and are emptied as
-- each record is moved across by the one-off migration task. Dropping them here
-- would destroy identities that have not been moved yet; leaving them
-- populated would mean the same identity in two places, one of them plaintext.
COMMENT ON COLUMN agents.nik IS 'Legacy plaintext; moved to kyc_records and cleared by the kyc:migrate task. Do not write here.';
COMMENT ON COLUMN pilgrims.nik IS 'Legacy plaintext; moved to kyc_records and cleared by the kyc:migrate task. Do not write here.';

-- +goose Down
DROP TABLE kyc_records;
