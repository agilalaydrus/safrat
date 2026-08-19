-- +goose Up
CREATE TABLE vendor_contracts (
  id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id           UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id             UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  vendor_name           TEXT        NOT NULL,
  vendor_type           TEXT        NOT NULL DEFAULT 'HOTEL'
                                    CHECK (vendor_type IN ('HOTEL','TRANSPORT','CATERING','VISA_AGENT','INSURANCE','OTHER')),
  contract_number       TEXT        NOT NULL DEFAULT '',
  committed_units       INTEGER     NOT NULL DEFAULT 0,
  confirmed_units       INTEGER     NOT NULL DEFAULT 0,
  confirmation_deadline DATE,
  rate_per_unit_idr     BIGINT      NOT NULL DEFAULT 0,
  total_value_idr       BIGINT      GENERATED ALWAYS AS (committed_units * rate_per_unit_idr) STORED,
  deposit_amount_idr    BIGINT      NOT NULL DEFAULT 0,
  deposit_paid          BOOLEAN     NOT NULL DEFAULT false,
  status                TEXT        NOT NULL DEFAULT 'NEGOTIATING'
                                    CHECK (status IN ('NEGOTIATING','CONFIRMED','PARTIAL','CANCELLED')),
  notes                 TEXT        NOT NULL DEFAULT '',
  contact_name          TEXT        NOT NULL DEFAULT '',
  contact_phone         TEXT        NOT NULL DEFAULT '',
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX vendor_contracts_operator_season_idx ON vendor_contracts(operator_id, season_id);

CREATE TRIGGER vendor_contracts_set_updated_at
  BEFORE UPDATE ON vendor_contracts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Immutable SLA event log — every status change or note is recorded here,
-- never updated after insert.
CREATE TABLE vendor_contract_events (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id     UUID        NOT NULL REFERENCES vendor_contracts(id) ON DELETE CASCADE,
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  event_type      TEXT        NOT NULL,
  description     TEXT        NOT NULL,
  recorded_by     TEXT        NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX vendor_contract_events_contract_idx ON vendor_contract_events(contract_id);

-- +goose Down
DROP TABLE IF EXISTS vendor_contract_events;
DROP TABLE IF EXISTS vendor_contracts;
