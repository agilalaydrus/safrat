-- +goose Up
-- Room tiers, and the seats behind them.
--
-- A package is not one price. The same umrah costs less four to a room than two
-- to a room, and every travel agency sells it that way — so a catalogue that
-- can only hold one number forces them to create three near-identical packages
-- and keep them in step by hand.
--
-- The tier price is stored as a DIFFERENCE from the package's base, never as an
-- absolute. Prices in this system are computed on read and never stored (see
-- ListProductPricing): a stored total is a copy that goes stale the moment the
-- base or a markup moves, and the two then disagree with nobody able to say
-- which is authoritative. A difference survives all of that.
CREATE TABLE product_room_tiers (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id   UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  -- Carried rather than joined for it. Every read in this codebase is scoped by
  -- operator at the repository layer, and a table that cannot be scoped without
  -- a join is a table somebody will eventually query without one.
  operator_id  UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  tier         TEXT NOT NULL CHECK (tier IN ('QUAD', 'TRIPLE', 'DOUBLE')),
  -- Signed on purpose. Quad is usually cheaper than the package's headline
  -- price and Double dearer, so both directions are ordinary.
  price_delta_idr BIGINT NOT NULL DEFAULT 0,
  -- NULL means no limit for this tier, which is different from zero. Zero is a
  -- tier that exists but has nothing left to sell — the two must never render
  -- the same, and NULL keeps "unlimited" from being spelled as a large number.
  seat_quota   INTEGER CHECK (seat_quota IS NULL OR seat_quota >= 0),
  is_active    BOOLEAN NOT NULL DEFAULT true,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (product_id, tier)
);

CREATE INDEX product_room_tiers_operator_idx ON product_room_tiers (operator_id, product_id);

CREATE TRIGGER product_room_tiers_set_updated_at
  BEFORE UPDATE ON product_room_tiers
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Which tier a sale consumed. NULL for everything that is not a travel package
-- — equipment and top-ups have no rooms — so it cannot be NOT NULL.
ALTER TABLE orders ADD COLUMN room_tier TEXT
  CHECK (room_tier IS NULL OR room_tier IN ('QUAD', 'TRIPLE', 'DOUBLE'));

-- The count behind every quota check. Cancelled, expired and failed orders
-- release their seat; everything else holds one.
CREATE INDEX orders_room_tier_idx ON orders (product_id, room_tier)
  WHERE room_tier IS NOT NULL AND status NOT IN ('CANCELLED', 'EXPIRED', 'FAILED', 'REFUNDED');

-- +goose StatementBegin
-- A quota nothing enforces is a number on a screen.
--
-- Enforced in the database rather than in the service for the same reason the
-- entitlement limits are: there is more than one path that creates an order
-- (dashboard, public checkout, agent, manual), and a check in one of them is a
-- check missing from the others.
CREATE OR REPLACE FUNCTION assert_room_tier_quota() RETURNS trigger AS $$
DECLARE
  quota INTEGER;
  taken INTEGER;
BEGIN
  IF NEW.room_tier IS NULL THEN
    RETURN NEW;
  END IF;
  -- Statuses that do not hold a seat never need checking, so a cancellation can
  -- always be recorded even when the tier is full.
  IF NEW.status IN ('CANCELLED', 'EXPIRED', 'FAILED', 'REFUNDED') THEN
    RETURN NEW;
  END IF;

  SELECT t.seat_quota INTO quota
  FROM product_room_tiers t
  WHERE t.product_id = NEW.product_id AND t.tier = NEW.room_tier AND t.is_active;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'room tier % is not offered for product %', NEW.room_tier, NEW.product_id
      USING ERRCODE = 'check_violation', CONSTRAINT = 'order_room_tier_offered';
  END IF;
  IF quota IS NULL THEN
    RETURN NEW;
  END IF;

  -- Serialised per tier. Without this two concurrent checkouts both count the
  -- same free seat and both are allowed — the classic oversell, and the reason
  -- counting inside a transaction is not enough on its own.
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.product_id::text || ':' || NEW.room_tier, 0));

  SELECT COUNT(*) INTO taken
  FROM orders o
  WHERE o.product_id = NEW.product_id
    AND o.room_tier = NEW.room_tier
    AND o.status NOT IN ('CANCELLED', 'EXPIRED', 'FAILED', 'REFUNDED')
    AND o.id IS DISTINCT FROM NEW.id;

  IF taken >= quota THEN
    RAISE EXCEPTION 'room tier % for product % is full (% of %)', NEW.room_tier, NEW.product_id, taken, quota
      USING ERRCODE = 'check_violation', CONSTRAINT = 'order_room_tier_quota';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER orders_assert_room_tier_quota
  BEFORE INSERT OR UPDATE OF room_tier, status, product_id ON orders
  FOR EACH ROW EXECUTE FUNCTION assert_room_tier_quota();

-- +goose Down
DROP TRIGGER IF EXISTS orders_assert_room_tier_quota ON orders;
DROP FUNCTION IF EXISTS assert_room_tier_quota();
DROP INDEX IF EXISTS orders_room_tier_idx;
ALTER TABLE orders DROP COLUMN IF EXISTS room_tier;
DROP TABLE IF EXISTS product_room_tiers;
