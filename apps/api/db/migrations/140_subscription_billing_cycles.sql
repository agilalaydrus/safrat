-- +goose Up
-- One commercial period may be billed only once per operator.
--
-- A pending-invoice constraint is not enough: after an invoice expires, a
-- retried batch could otherwise create a second historical charge for the
-- same access period. period_start is the subscription's stable access_until
-- captured by the preview, so it remains the same across retries and workers.
CREATE UNIQUE INDEX subscription_invoices_operator_period_idx
  ON subscription_invoices (operator_id, period_start);

-- +goose Down
DROP INDEX IF EXISTS subscription_invoices_operator_period_idx;
