-- +goose Up

-- Nothing recorded what a product costs to supply, so nothing could tell that
-- a price was below it. Selling at a loss looked exactly like selling at a
-- profit, and the margin split was taken out of a number that had no floor
-- underneath it.
--
-- Two ways to know the cost, because digital and non-digital products learn it
-- differently:
--
--   set by hand   — an operator entering what they pay a supplier, in the
--                   product's routing settings.
--   observed      — read back from the supplier's own response the first time
--                   a transaction actually goes through, and refreshed on
--                   every later one, so it cannot drift out of date silently.
--
-- Whichever it came from, the price is validated against it before a sale.
ALTER TABLE products
  ADD COLUMN supplier_cost_idr BIGINT CHECK (supplier_cost_idr IS NULL OR supplier_cost_idr >= 0),
  -- MANUAL | OBSERVED. Kept because the two carry different weight: an
  -- observed cost is what the supplier actually charged, a manual one is what
  -- somebody typed, and an observed value must never be silently overwritten
  -- by a stale manual entry.
  ADD COLUMN supplier_cost_source TEXT NOT NULL DEFAULT ''
    CHECK (supplier_cost_source IN ('', 'MANUAL', 'OBSERVED')),
  ADD COLUMN supplier_cost_updated_at TIMESTAMPTZ;

-- A cost with no source, or a source with no cost, would mean neither can be
-- trusted. Keep them consistent at the row level rather than hoping.
ALTER TABLE products ADD CONSTRAINT products_supplier_cost_consistent_check
  CHECK ((supplier_cost_idr IS NULL) = (supplier_cost_source = ''));

-- What a supplier actually charged, per transaction. The product carries the
-- latest figure for validation; this keeps the history behind it, so a cost
-- that moves can be seen moving rather than just being different today.
CREATE TABLE supplier_cost_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
  cost_idr BIGINT NOT NULL CHECK (cost_idr >= 0),
  -- The supplier's own reference for the purchase, so an observation can be
  -- traced back to something outside this system.
  supplier_reference TEXT NOT NULL DEFAULT '',
  observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX supplier_cost_observations_product_idx
  ON supplier_cost_observations (product_id, observed_at DESC);

-- One observation per order: a retried fulfilment reports the same purchase,
-- not a second one.
CREATE UNIQUE INDEX supplier_cost_observations_order_idx
  ON supplier_cost_observations (order_id) WHERE order_id IS NOT NULL;

-- Observations are evidence of what was charged, so they are history like
-- every other money record here.
CREATE TRIGGER supplier_cost_observations_append_only
  BEFORE UPDATE OR DELETE ON supplier_cost_observations
  FOR EACH ROW EXECUTE FUNCTION ledger_is_append_only();

-- +goose Down
DROP TABLE supplier_cost_observations;
ALTER TABLE products DROP CONSTRAINT products_supplier_cost_consistent_check;
ALTER TABLE products
  DROP COLUMN supplier_cost_updated_at,
  DROP COLUMN supplier_cost_source,
  DROP COLUMN supplier_cost_idr;
