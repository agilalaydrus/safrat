-- +goose Up

-- A cap of Rp20.000.000 per account per day on digital products (owner's
-- rule). The hard part is not the number, it is that a cap checked by reading
-- a total and then writing an order is not a cap at all: two requests arriving
-- together both read the old total, both find room, and both write. The
-- account ends the day over the limit and nothing in the logs looks wrong.
--
-- So the total lives in a row with a CHECK on it, and spending is an UPSERT
-- that increments in place. Postgres evaluates the constraint against the
-- incremented value, under the row lock the UPDATE already takes. Two
-- concurrent purchases serialise; whichever lands second sees the first's
-- total and is refused by the database, not by a code path that has to
-- remember to look.

CREATE TABLE daily_digital_spend (
  -- PILGRIM or AGENT, matching orders.buyer_kind. An agent and a pilgrim could
  -- in principle share an id from different tables, and silently merging their
  -- limits would let one party's spending block another's.
  buyer_kind TEXT NOT NULL CHECK (buyer_kind IN ('PILGRIM','AGENT')),
  buyer_id   UUID NOT NULL,

  -- The calendar day in Asia/Jakarta, not UTC. A UTC day would roll over at
  -- 07:00 local time, so an evening purchase and the next morning's would
  -- count against different days while a single afternoon spans two — nobody
  -- would be able to predict when their limit resets.
  spend_date DATE NOT NULL,

  total_idr  BIGINT NOT NULL DEFAULT 0,

  -- Carried per row rather than hard-coded in the constraint so a single
  -- account can be raised without a migration, and so the limit in force is
  -- visible next to the total it applies to.
  limit_idr  BIGINT NOT NULL DEFAULT 20000000,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  PRIMARY KEY (buyer_kind, buyer_id, spend_date),

  -- The cap itself. Violating this is how the limit is enforced, so the code
  -- reads a unique-violation-shaped error and turns it into a refusal.
  CONSTRAINT daily_digital_spend_within_limit CHECK (total_idr <= limit_idr),

  -- Reversals must not drive a day negative. A total below zero would silently
  -- create headroom that was never earned — refunding an order twice, or
  -- reversing one that was never counted, would hand the account extra limit.
  CONSTRAINT daily_digital_spend_nonnegative CHECK (total_idr >= 0)
);

CREATE TRIGGER daily_digital_spend_set_updated_at
  BEFORE UPDATE ON daily_digital_spend
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Which orders have been counted, so a reversal cannot fire twice and a retry
-- cannot count twice. Without this, idempotency would depend on the caller
-- getting it right every time, across three settlement paths and a sweep.
--
-- Nullable: an order that never counted (non-digital, or created before this
-- existed) carries NULL and is skipped by every reversal.
ALTER TABLE orders ADD COLUMN digital_spend_counted_on DATE;

-- Backfilled so existing digital orders count toward their own day rather than
-- appearing as free headroom. Only orders that still hold value: a refunded or
-- failed order was never meant to consume limit.
UPDATE orders o
SET digital_spend_counted_on = (o.created_at AT TIME ZONE 'Asia/Jakarta')::date
WHERE o.status IN ('PENDING','HELD','PAID')
  AND EXISTS (
    SELECT 1 FROM products p
    WHERE p.id = o.product_id AND p.category IN ('ROAMING_DATA','PPOB_CREDIT')
  );

-- Seed the totals from that backfill. Sums can exceed the cap for accounts that
-- already spent more than Rp20 juta today, so the constraint is added only
-- after seeding — an existing overage is a fact to record, not a reason to
-- refuse the migration. Those accounts simply cannot spend more until tomorrow.
ALTER TABLE daily_digital_spend DROP CONSTRAINT daily_digital_spend_within_limit;

INSERT INTO daily_digital_spend (buyer_kind, buyer_id, spend_date, total_idr)
SELECT o.buyer_kind,
       COALESCE(o.pilgrim_id, o.buyer_agent_id),
       o.digital_spend_counted_on,
       SUM(o.total_price_idr)
FROM orders o
WHERE o.digital_spend_counted_on IS NOT NULL
GROUP BY 1, 2, 3
ON CONFLICT DO NOTHING;

ALTER TABLE daily_digital_spend
  ADD CONSTRAINT daily_digital_spend_within_limit CHECK (total_idr <= limit_idr);

-- +goose Down
ALTER TABLE orders DROP COLUMN IF EXISTS digital_spend_counted_on;
DROP TABLE IF EXISTS daily_digital_spend;
