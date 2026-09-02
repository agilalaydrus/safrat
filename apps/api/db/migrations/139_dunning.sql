-- +goose Up
-- Chasing an unpaid subscription, and what happens when nobody answers.
--
-- Renewal invoices already issue themselves seven days ahead (subscription
-- sweep). What did not exist was anything after that: an invoice went PAST_DUE
-- and sat there. Recurring revenue stopped quietly, which is the worst way for
-- it to stop.

-- Numbers a person changes, not constants a deploy changes. Trial length lives
-- here too: three days was in Go source, and a commercial figure that needs a
-- release to correct stays wrong for months.
CREATE TABLE platform_settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_by TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO platform_settings (key, value) VALUES
  -- Owner's decision, 2 September 2026. Long enough to cross a weekend, import
  -- a spreadsheet, and put one real registration through.
  ('trial_days', '10'),
  -- Days after due_at that each reminder goes out.
  ('dunning_days', '1,7,14'),
  -- When access stops being extended. Data is untouched; only entry closes.
  ('suspend_after_days', '21');

-- One row per stage per lapse, and the primary key is what makes that true.
--
-- Keyed on the lapse rather than on an invoice, which took a second attempt to
-- get right. An unpaid invoice does not survive long enough to chase: it is
-- issued seven days before access ends, expires exactly when access ends (so
-- its unique transfer amount returns to the pool), and the sweep then issues a
-- fresh one. Anchored on invoice_id, the H+7 reminder would be chasing a row
-- that no longer exists while a different invoice carried the debt.
--
-- lapsed_at is the access_until that ran out, so it is stable for the whole
-- episode and changes the moment payment pushes access forward — which starts a
-- clean sequence for any future lapse without anything having to be reset.
--
-- The outbox delivers at least once, so a worker that runs twice must not send
-- a second warning to the same agency: the insert collides and does nothing.
-- A customer who receives the same demand twice stops believing the third.
CREATE TABLE dunning_log (
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  lapsed_at   TIMESTAMPTZ NOT NULL,
  stage       TEXT NOT NULL CHECK (stage IN ('H1', 'H7', 'H14', 'SUSPEND')),
  sent_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  channel     TEXT NOT NULL DEFAULT 'EMAIL',
  PRIMARY KEY (operator_id, lapsed_at, stage)
);
CREATE INDEX dunning_log_sent_idx ON dunning_log (sent_at DESC);

-- Access is already governed by access_until, and this does not change that.
-- It only records *why* entry is closed, so "has not paid yet" can be told
-- apart from "deliberately frozen" — the two need different conversations.
ALTER TABLE subscriptions ADD COLUMN suspended_at TIMESTAMPTZ;
ALTER TABLE subscriptions ADD COLUMN suspended_reason TEXT NOT NULL DEFAULT '';
CREATE INDEX subscriptions_suspended_idx ON subscriptions (suspended_at) WHERE suspended_at IS NOT NULL;

-- A voided invoice keeps its row. Deleting one would put a hole in the billing
-- history exactly where somebody will later look, and CANCELLED already
-- releases the unique transfer amount back to the pool.
ALTER TABLE subscription_invoices ADD COLUMN voided_at TIMESTAMPTZ;
ALTER TABLE subscription_invoices ADD COLUMN voided_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE subscription_invoices ADD CONSTRAINT subscription_invoices_void_check
  CHECK (voided_at IS NULL OR status = 'CANCELLED');

-- Evidence, not cache: the same treatment migration 125 gives audit_logs.
-- Whether a warning was sent, and when, is exactly what a billing dispute
-- turns on.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    EXECUTE 'REVOKE UPDATE, DELETE, TRUNCATE ON dunning_log FROM safrat_app';
    EXECUTE 'GRANT SELECT, INSERT ON dunning_log TO safrat_app';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE ON platform_settings TO safrat_app';
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE subscription_invoices DROP CONSTRAINT IF EXISTS subscription_invoices_void_check;
ALTER TABLE subscription_invoices DROP COLUMN IF EXISTS voided_reason;
ALTER TABLE subscription_invoices DROP COLUMN IF EXISTS voided_at;
DROP INDEX IF EXISTS subscriptions_suspended_idx;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS suspended_reason;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS suspended_at;
DROP TABLE IF EXISTS dunning_log;
DROP TABLE IF EXISTS platform_settings;
