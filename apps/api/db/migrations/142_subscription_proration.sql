-- +goose Up
-- Mutable balance is operational state; this append-only table is the evidence
-- from which every proration decision can be reconstructed.
ALTER TABLE subscriptions
  ADD COLUMN credit_balance_idr BIGINT NOT NULL DEFAULT 0
    CHECK (credit_balance_idr >= 0);

ALTER TABLE subscription_invoices
  ADD COLUMN purpose TEXT NOT NULL DEFAULT 'RENEWAL'
    CHECK (purpose IN ('RENEWAL','PRORATION'));

CREATE TABLE subscription_adjustments (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id         UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  invoice_id          UUID UNIQUE REFERENCES subscription_invoices(id),
  kind                TEXT NOT NULL CHECK (kind IN ('PRORATION_DEBIT','PRORATION_CREDIT')),
  from_plan           plan NOT NULL,
  to_plan             plan NOT NULL,
  amount_idr          BIGINT NOT NULL CHECK (amount_idr <> 0),
  effective_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  access_until_snapshot TIMESTAMPTZ NOT NULL,
  remaining_seconds   INTEGER NOT NULL CHECK (remaining_seconds > 0),
  period_seconds      INTEGER NOT NULL CHECK (period_seconds > 0),
  reason              TEXT NOT NULL CHECK (length(trim(reason)) > 0),
  requested_by        TEXT NOT NULL CHECK (length(trim(requested_by)) > 0),
  idempotency_key     TEXT NOT NULL CHECK (length(trim(idempotency_key)) BETWEEN 8 AND 128),
  request_fingerprint TEXT NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (requested_by, idempotency_key)
);
CREATE INDEX subscription_adjustments_operator_idx
  ON subscription_adjustments (operator_id, created_at DESC);

-- Evidence, not cache. Corrections are another adjustment in the opposite
-- direction; the application may never rewrite the original decision.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE UPDATE, DELETE, TRUNCATE ON subscription_adjustments FROM safrat_app;
    GRANT SELECT, INSERT ON subscription_adjustments TO safrat_app;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS subscription_adjustments;
ALTER TABLE subscription_invoices DROP COLUMN IF EXISTS purpose;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS credit_balance_idr;
