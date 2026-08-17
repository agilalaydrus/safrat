-- +goose Up
-- Module 7: Digital Products & Orders. Margins live on the product, not
-- the order — every order of the same product splits the same way, and
-- validating "platformMargin + operatorMargin + agentMargin <= 1.0" once
-- at product-save time (CODEX_SPEC.md §7) is simpler than re-validating
-- per order. Defaults are a starting point, not a business decision baked
-- in — every product can override them.
ALTER TABLE products
  ADD COLUMN platform_margin_pct FLOAT8 NOT NULL DEFAULT 0.15 CHECK (platform_margin_pct >= 0),
  ADD COLUMN operator_margin_pct FLOAT8 NOT NULL DEFAULT 0.70 CHECK (operator_margin_pct >= 0),
  ADD COLUMN agent_margin_pct    FLOAT8 NOT NULL DEFAULT 0.15 CHECK (agent_margin_pct >= 0),
  ADD CONSTRAINT products_margins_sum_check CHECK (platform_margin_pct + operator_margin_pct + agent_margin_pct <= 1.0);

CREATE TABLE orders (
  id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id          UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id            UUID        NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  pilgrim_id           UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  product_id           UUID        NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  -- Nullable, and deliberately not defaulted from the pilgrim's own
  -- agent_id — the agent who gets commission is whoever sold *this*
  -- order, not necessarily whoever originally referred the pilgrim.
  agent_id             UUID        REFERENCES agents(id) ON DELETE SET NULL,
  quantity             INT         NOT NULL DEFAULT 1 CHECK (quantity > 0),
  unit_price_idr       BIGINT      NOT NULL,
  total_price_idr      BIGINT      NOT NULL,
  -- Split of total_price_idr at order-creation time, per the product's
  -- margins at that moment — frozen on the order so a later product edit
  -- never rewrites a past order's numbers. agent_commission_idr is 0
  -- whenever agent_id is NULL (CODEX_SPEC.md §7: "agentCommission = 0
  -- when there's no agentId"), enforced in Go, not just by convention.
  platform_amount_idr  BIGINT      NOT NULL DEFAULT 0,
  operator_amount_idr  BIGINT      NOT NULL DEFAULT 0,
  agent_commission_idr BIGINT      NOT NULL DEFAULT 0,
  status               TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED','CANCELLED')),
  xendit_invoice_id    TEXT,
  xendit_invoice_url   TEXT,
  paid_at              TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX orders_operator_season_idx ON orders(operator_id, season_id);
CREATE INDEX orders_pilgrim_idx ON orders(pilgrim_id);
CREATE UNIQUE INDEX orders_xendit_invoice_idx ON orders(xendit_invoice_id) WHERE xendit_invoice_id IS NOT NULL;
CREATE TRIGGER orders_set_updated_at
  BEFORE UPDATE ON orders
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS orders;
ALTER TABLE products
  DROP CONSTRAINT IF EXISTS products_margins_sum_check,
  DROP COLUMN IF EXISTS platform_margin_pct,
  DROP COLUMN IF EXISTS operator_margin_pct,
  DROP COLUMN IF EXISTS agent_margin_pct;
