-- +goose Up
-- Operator subscriptions and their invoices.
--
-- Until now `plan` was a label with nothing behind it: no billing, no period,
-- no way to change it except UPDATE by hand. Prices are published, so the
-- product needs to be able to charge for them.

CREATE TYPE subscription_status AS ENUM ('TRIALING', 'ACTIVE', 'PAST_DUE', 'CANCELLED');
CREATE TYPE invoice_status AS ENUM ('PENDING', 'PAID', 'EXPIRED', 'CANCELLED');
-- BANK_TRANSFER is matched by a unique amount; GATEWAY is Xendit's hosted
-- checkout, which already covers QRIS, cards, VA and e-wallets.
CREATE TYPE payment_channel AS ENUM ('BANK_TRANSFER', 'GATEWAY');

-- Prices live in the database rather than in code so they can be corrected
-- without a deploy. Keep them in step with PRICING_TIERS in
-- apps/web/components/landing/content.ts — the landing page still states its
-- own figures, and a mismatch would quote one price and invoice another.
CREATE TABLE plan_prices (
  plan            plan PRIMARY KEY,
  monthly_idr     BIGINT NOT NULL CHECK (monthly_idr > 0),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO plan_prices (plan, monthly_idr) VALUES
  ('STARTER', 589000),
  ('GROWTH',  789000),
  ('PRO',     2489000);

CREATE TABLE subscriptions (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- One subscription per operator: two would make "is this operator paid up?"
  -- ambiguous, which is the one question the whole table exists to answer.
  operator_id        UUID NOT NULL UNIQUE REFERENCES operators(id) ON DELETE CASCADE,
  plan               plan NOT NULL,
  status             subscription_status NOT NULL DEFAULT 'TRIALING',
  -- Access is granted strictly by time, never by status alone, so a stale
  -- status can never hand out free access: see access_until.
  access_until       TIMESTAMPTZ NOT NULL,
  cancelled_at       TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX subscriptions_access_idx ON subscriptions (access_until);

CREATE TABLE subscription_invoices (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  plan            plan NOT NULL,
  status          invoice_status NOT NULL DEFAULT 'PENDING',
  channel         payment_channel NOT NULL,
  base_amount_idr BIGINT NOT NULL CHECK (base_amount_idr > 0),
  -- What the operator must actually send. For BANK_TRANSFER this carries a
  -- unique suffix, and that suffix is the ONLY thing tying an incoming bank
  -- mutation to this invoice.
  amount_idr      BIGINT NOT NULL CHECK (amount_idr > 0),
  period_start    TIMESTAMPTZ NOT NULL,
  period_end      TIMESTAMPTZ NOT NULL CHECK (period_end > period_start),
  due_at          TIMESTAMPTZ NOT NULL,
  paid_at         TIMESTAMPTZ,
  -- Xendit's invoice id and hosted checkout URL, for GATEWAY invoices.
  external_id     TEXT,
  checkout_url    TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK ((status = 'PAID') = (paid_at IS NOT NULL))
);
CREATE INDEX subscription_invoices_operator_idx ON subscription_invoices (operator_id, created_at DESC);
CREATE INDEX subscription_invoices_due_idx ON subscription_invoices (status, due_at);

-- The correctness backbone of unique-amount matching. Two unpaid bank
-- transfers for the same rupiah figure would make an incoming mutation
-- impossible to attribute, and money would be credited to the wrong travel
-- agency. The database refuses to let that state exist.
CREATE UNIQUE INDEX subscription_invoices_transfer_amount_idx
  ON subscription_invoices (amount_idr)
  WHERE status = 'PENDING' AND channel = 'BANK_TRANSFER';

-- Xendit's id must map to exactly one invoice, or a webhook could settle the
-- wrong one.
CREATE UNIQUE INDEX subscription_invoices_external_idx
  ON subscription_invoices (external_id) WHERE external_id IS NOT NULL;

-- Existing operators are already live and were never charged; putting them on
-- a 3-day trial would lock them out on Monday. They start ACTIVE with a
-- generous runway, to be reconciled deliberately rather than by migration.
INSERT INTO subscriptions (operator_id, plan, status, access_until)
SELECT id, plan, 'ACTIVE', NOW() + INTERVAL '90 days' FROM operators;

-- +goose Down
DROP TABLE IF EXISTS subscription_invoices;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plan_prices;
DROP TYPE IF EXISTS payment_channel;
DROP TYPE IF EXISTS invoice_status;
DROP TYPE IF EXISTS subscription_status;
