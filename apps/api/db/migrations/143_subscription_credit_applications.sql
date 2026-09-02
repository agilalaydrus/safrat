-- +goose Up
ALTER TABLE subscription_invoices
  ADD COLUMN credit_applied_idr BIGINT NOT NULL DEFAULT 0,
  ADD CONSTRAINT subscription_invoices_credit_check
    CHECK (credit_applied_idr >= 0 AND credit_applied_idr < base_amount_idr);

-- Credit is reserved in the invoice amount but consumed only on payment.
-- This immutable row is the evidence that a balance reduction corresponded
-- to money settling a particular invoice.
CREATE TABLE subscription_credit_applications (
  invoice_id    UUID PRIMARY KEY REFERENCES subscription_invoices(id),
  operator_id   UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  amount_idr    BIGINT NOT NULL CHECK (amount_idr > 0),
  applied_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE UPDATE, DELETE, TRUNCATE ON subscription_credit_applications FROM safrat_app;
    GRANT SELECT, INSERT ON subscription_credit_applications TO safrat_app;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS subscription_credit_applications;
ALTER TABLE subscription_invoices DROP CONSTRAINT IF EXISTS subscription_invoices_credit_check;
ALTER TABLE subscription_invoices DROP COLUMN IF EXISTS credit_applied_idr;
