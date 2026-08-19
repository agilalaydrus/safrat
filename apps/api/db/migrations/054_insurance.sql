-- +goose Up
ALTER TABLE pilgrims
  ADD COLUMN insurance_provider  TEXT NOT NULL DEFAULT '',
  ADD COLUMN insurance_policy_no TEXT NOT NULL DEFAULT '',
  ADD COLUMN insurance_class     TEXT NOT NULL DEFAULT 'STANDARD'
                           CHECK (insurance_class IN ('STANDARD','PLUS','PREMIUM')),
  ADD COLUMN blood_type          TEXT NOT NULL DEFAULT '',
  ADD COLUMN chronic_conditions  TEXT NOT NULL DEFAULT '',
  ADD COLUMN current_medications TEXT NOT NULL DEFAULT '';

-- Tracks insurance claim events — immutable log; only status/settled_amount
-- ever change after filing, never the original claim facts.
CREATE TABLE insurance_claims (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id      UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  claim_type      TEXT        NOT NULL CHECK (claim_type IN ('MEDICAL','DEATH','FLIGHT','BAGGAGE','OTHER')),
  incident_date   DATE        NOT NULL,
  description     TEXT        NOT NULL,
  status          TEXT        NOT NULL DEFAULT 'FILED'
                              CHECK (status IN ('FILED','SUBMITTED','PROCESSING','SETTLED','REJECTED')),
  claim_amount_idr   BIGINT,
  settled_amount_idr BIGINT,
  filed_by        TEXT        NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX insurance_claims_operator_idx ON insurance_claims(operator_id);
CREATE INDEX insurance_claims_pilgrim_idx  ON insurance_claims(pilgrim_id);

CREATE TRIGGER insurance_claims_set_updated_at
  BEFORE UPDATE ON insurance_claims
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS insurance_claims;
ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS insurance_provider,
  DROP COLUMN IF EXISTS insurance_policy_no,
  DROP COLUMN IF EXISTS insurance_class,
  DROP COLUMN IF EXISTS blood_type,
  DROP COLUMN IF EXISTS chronic_conditions,
  DROP COLUMN IF EXISTS current_medications;
