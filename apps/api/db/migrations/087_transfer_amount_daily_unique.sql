-- +goose Up
-- Strengthens unique-amount matching from "unique among unpaid" to "unique per
-- day, regardless of status".
--
-- The previous rule released an amount the moment its invoice was settled, so
-- a code paid at 09:00 could be reissued to a different operator at 10:00. A
-- bank mutation arriving later that day carries a date, not a precise instant,
-- and matching is by amount — so that mutation could be credited to either
-- invoice. Same-day reuse is therefore unsafe even though neither invoice is
-- pending at the same time.
--
-- Both constraints are kept and do different jobs:
--   * the pending index stops two live invoices sharing an amount at all;
--   * this one stops an amount being reused within the same day.
-- Together, a code becomes free only once its day has passed and nothing
-- unpaid still holds it.

-- Jakarta, not UTC: "the same day" has to mean the operator's day and the
-- bank statement's day, not the server's.
ALTER TABLE subscription_invoices
  ADD COLUMN transfer_day date
  GENERATED ALWAYS AS ((created_at AT TIME ZONE 'Asia/Jakarta')::date) STORED;

CREATE UNIQUE INDEX subscription_invoices_transfer_daily_idx
  ON subscription_invoices (transfer_day, amount_idr)
  WHERE channel = 'BANK_TRANSFER';

-- +goose Down
DROP INDEX IF EXISTS subscription_invoices_transfer_daily_idx;
ALTER TABLE subscription_invoices DROP COLUMN IF EXISTS transfer_day;
