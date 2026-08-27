-- +goose Up

-- Digital products are supplied by TawafiqHub, not by each travel (owner's
-- ruling). Nothing in the schema expressed that: there was no supplier, no way
-- to say which supplier fulfils which product, and no record of what a supplier
-- actually said when asked. Fulfilment for ROAMING_DATA and PPOB_CREDIT does
-- not exist at all, and this is the shape it needs to exist in.
--
-- Everything here is platform-owned. No operator_id anywhere: a travel does not
-- get to point a product at a different supplier, or see another travel's
-- routing. Access is the platform_admins gate.

CREATE TABLE suppliers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  -- Stable machine name used in logs and in routing, so renaming a supplier
  -- for display does not orphan its history.
  code TEXT NOT NULL UNIQUE,
  base_url TEXT NOT NULL DEFAULT '',
  -- Credentials are never stored here. This names the environment variable the
  -- worker reads them from, so a database dump carries no secrets and rotating
  -- a key never touches a row.
  credential_env_var TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'INACTIVE' CHECK (status IN ('ACTIVE', 'INACTIVE', 'SUSPENDED')),
  notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Which supplier fulfils a product, and what that product is called on their
-- side. One route per product: two active routes would make "which supplier
-- did this sale go to" unanswerable after the fact.
CREATE TABLE product_routes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL UNIQUE REFERENCES products(id) ON DELETE CASCADE,
  supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
  -- The supplier's own identifier for the item, e.g. a denomination code.
  supplier_sku TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX product_routes_supplier_idx ON product_routes (supplier_id);

-- How to read a supplier's answer.
--
-- Suppliers in this market answer in wildly different shapes — JSON, form
-- bodies, plain SMS-style text — and change them without warning. Hard-coding a
-- parser per supplier means a code deploy every time one shifts a field. These
-- are ordered patterns applied to the raw response instead, editable from the
-- admin panel.
--
-- Deliberately not a general scripting hook: a regex can only read, so a bad
-- rule produces a wrong status, never arbitrary execution inside the worker.
CREATE TABLE supplier_response_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
  -- Lower runs first; the first match decides. Ties broken by created_at so
  -- ordering is always total and never depends on row order.
  priority INT NOT NULL DEFAULT 100,
  -- RE2 (Go's regexp). No backreferences and no lookaround, which is also why
  -- a hostile pattern cannot hang the worker.
  pattern TEXT NOT NULL,
  -- What a match means.
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'FAILED', 'PENDING')),
  -- Named capture groups to lift out of the match, when present:
  --   reference — the supplier's transaction id, for tracing a dispute
  --   cost      — what they charged, feeding supplier_cost_observations
  -- Both optional: plenty of responses carry neither.
  reference_group TEXT NOT NULL DEFAULT '',
  cost_group TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX supplier_response_rules_supplier_idx
  ON supplier_response_rules (supplier_id, priority, created_at);

-- Every exchange with a supplier, kept whole.
--
-- This is the only record of what was actually asked and answered when a
-- fulfilment is disputed — by a jamaah who paid, or by the supplier. Parsed
-- fields are stored alongside the raw body rather than instead of it, so a rule
-- that turns out to be wrong can be re-read against what really arrived.
CREATE TABLE supplier_request_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
  order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
  operator_id UUID REFERENCES operators(id) ON DELETE SET NULL,
  direction TEXT NOT NULL CHECK (direction IN ('REQUEST', 'CALLBACK')),
  endpoint TEXT NOT NULL DEFAULT '',
  -- Bodies as sent and received. Credentials must be redacted before writing:
  -- they belong in an env var, and a log that carries them turns every dump
  -- into a credential leak.
  request_body TEXT NOT NULL DEFAULT '',
  response_body TEXT NOT NULL DEFAULT '',
  http_status INT,
  -- What the rules made of it, and which rule decided.
  outcome TEXT NOT NULL DEFAULT '' CHECK (outcome IN ('', 'SUCCESS', 'FAILED', 'PENDING', 'UNMATCHED')),
  matched_rule_id UUID REFERENCES supplier_response_rules(id) ON DELETE SET NULL,
  supplier_reference TEXT NOT NULL DEFAULT '',
  cost_idr BIGINT,
  error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX supplier_request_logs_order_idx ON supplier_request_logs (order_id, created_at DESC);
CREATE INDEX supplier_request_logs_supplier_idx ON supplier_request_logs (supplier_id, created_at DESC);
-- Unmatched responses are the queue that keeps the rules honest: every one is a
-- supplier saying something nobody taught the system to read.
CREATE INDEX supplier_request_logs_unmatched_idx
  ON supplier_request_logs (created_at DESC) WHERE outcome = 'UNMATCHED';

-- A log is evidence. Same rule as the money records.
CREATE TRIGGER supplier_request_logs_append_only
  BEFORE UPDATE OR DELETE ON supplier_request_logs
  FOR EACH ROW EXECUTE FUNCTION ledger_is_append_only();

-- The application may read and add logs, never rewrite them.
REVOKE UPDATE, DELETE, TRUNCATE ON supplier_request_logs FROM safrat_app;

-- +goose Down
DROP TABLE supplier_request_logs;
DROP TABLE supplier_response_rules;
DROP TABLE product_routes;
DROP TABLE suppliers;
