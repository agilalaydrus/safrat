-- +goose Up

-- Margins were double precision, and every order's split was computed as
-- int64(float64(total) * pct). Money itself is BIGINT rupiah and exact, but
-- the multiplier was not: 0.70 has no exact binary representation, so the
-- product lands a hair under the true value and truncation drops a rupiah.
--
-- Measured, not assumed: across 25,070 price/margin combinations, 120 came out
-- one rupiah short, always in the same direction. Small per transaction, but
-- systematic and permanent — and it lands on the operator's share.
--
-- Basis points are integers (1500 = 15.00%), so total * bps / 10000 is exact
-- integer arithmetic from end to end. Two decimal places of precision, which
-- is more than any commission agreement here uses.
ALTER TABLE products
  ADD COLUMN platform_margin_bps INT NOT NULL DEFAULT 1500,
  ADD COLUMN operator_margin_bps INT NOT NULL DEFAULT 7000,
  ADD COLUMN agent_margin_bps    INT NOT NULL DEFAULT 1500;

-- ROUND, not truncate: the stored double is a hair below the intended
-- percentage, and truncating would bake that error into the new column.
UPDATE products SET
  platform_margin_bps = ROUND(platform_margin_pct * 10000),
  operator_margin_bps = ROUND(operator_margin_pct * 10000),
  agent_margin_bps    = ROUND(agent_margin_pct * 10000);

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_margins_sum_check;
ALTER TABLE products
  ADD CONSTRAINT products_margins_sum_check
  CHECK (platform_margin_bps + operator_margin_bps + agent_margin_bps <= 10000),
  ADD CONSTRAINT products_margins_nonnegative_check
  CHECK (platform_margin_bps >= 0 AND operator_margin_bps >= 0 AND agent_margin_bps >= 0);

ALTER TABLE products
  DROP COLUMN platform_margin_pct,
  DROP COLUMN operator_margin_pct,
  DROP COLUMN agent_margin_pct;

-- +goose Down
ALTER TABLE products
  ADD COLUMN platform_margin_pct FLOAT8 NOT NULL DEFAULT 0.15,
  ADD COLUMN operator_margin_pct FLOAT8 NOT NULL DEFAULT 0.70,
  ADD COLUMN agent_margin_pct    FLOAT8 NOT NULL DEFAULT 0.15;
UPDATE products SET
  platform_margin_pct = platform_margin_bps / 10000.0,
  operator_margin_pct = operator_margin_bps / 10000.0,
  agent_margin_pct    = agent_margin_bps / 10000.0;
ALTER TABLE products DROP CONSTRAINT products_margins_sum_check;
ALTER TABLE products DROP CONSTRAINT products_margins_nonnegative_check;
ALTER TABLE products
  ADD CONSTRAINT products_margins_sum_check
  CHECK (platform_margin_pct + operator_margin_pct + agent_margin_pct <= 1.0);
ALTER TABLE products
  DROP COLUMN platform_margin_bps,
  DROP COLUMN operator_margin_bps,
  DROP COLUMN agent_margin_bps;
