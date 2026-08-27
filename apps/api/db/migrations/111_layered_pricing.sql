-- +goose Up

-- Price was a single number typed into the product, and the margin split was
-- taken out of it downwards: platform 15%, operator 70%, agent 15% of whatever
-- somebody entered. That answers "how do we divide this price" but never
-- "where did this price come from", so nothing could tell a considered price
-- from a typo, and changing one level's share silently moved every other
-- level's money.
--
-- Inverted here. Price is built upwards from the platform's base, and each
-- level adds its own markup in rupiah:
--
--   base_price_idr             what TawafiqHub charges the travel
--   + operator_markup_idr      what the travel adds  -> the AGENT price
--   + agent_markup_idr         what the agent earns  -> the JAMAAH price
--
-- Two prices, one per kind of buyer. An agent buys at the agent price, which
-- is why buying on their own account earns them nothing: the agent markup is
-- simply not in the price they paid.
--
-- Every level exists on every product, at zero if nobody set it. A level that
-- is absent and a level that is deliberately zero must not look alike — the
-- first is a configuration gap and the second is a decision, and only one of
-- them needs chasing.

-- The platform's base. Nullable rather than zero-defaulted: a product whose
-- base has never been set has no price at all, and must be refused at sale
-- rather than sold for the markup alone.
--
-- Distinct from supplier_cost_idr, which is what TawafiqHub *pays*. The
-- difference between the two is the platform's own margin, and keeping them
-- separate is what makes that margin visible instead of implied.
ALTER TABLE products
  ADD COLUMN base_price_idr BIGINT CHECK (base_price_idr IS NULL OR base_price_idr >= 0);

-- Backfilled from the existing price so no product loses its price on the way
-- through. The old split becomes the starting markups, which reproduces each
-- product's current numbers exactly rather than resetting them to zero and
-- making every operator re-enter their catalogue.
UPDATE products SET base_price_idr =
  price_idr - (price_idr * operator_margin_bps / 10000)
            - (price_idr * agent_margin_bps / 10000)
WHERE base_price_idr IS NULL;

-- Markups live in their own table, one row per product per operator.
--
-- Separate from products because of where this is going: the digital
-- catalogue is moving to platform ownership, at which point one product is
-- shared by every travel and each still sets its own markup. Putting the
-- markup on the product would have to be undone to get there; putting it
-- beside the product does not.
CREATE TABLE product_markups (
  id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id          UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  operator_id         UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,

  -- What the travel adds over the platform base. Zero is a valid, meaningful
  -- setting: selling at the platform base to win the customer.
  operator_markup_idr BIGINT      NOT NULL DEFAULT 0 CHECK (operator_markup_idr >= 0),

  -- What sits on top for the agent level. Part of the jamaah price whether or
  -- not a referring agent exists — otherwise a jamaah who came in through an
  -- agent would pay more than one who walked in, and the agent's own customers
  -- would be the ones penalised for it. With no referrer this amount goes to
  -- the operator instead; the jamaah pays the same either way.
  agent_markup_idr    BIGINT      NOT NULL DEFAULT 0 CHECK (agent_markup_idr >= 0),

  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One markup row per product per operator, enforced here rather than by
-- reading first: two staff saving the pricing screen at once would otherwise
-- both pass the check and leave the product with two different prices, and
-- which one applies would come down to row order.
CREATE UNIQUE INDEX product_markups_product_operator_idx
  ON product_markups (product_id, operator_id);

CREATE TRIGGER product_markups_set_updated_at
  BEFORE UPDATE ON product_markups
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Every existing product gets its row, carrying the markups implied by the
-- split it already had. After this, "no markup row" means a genuinely new
-- product, not an unmigrated one.
INSERT INTO product_markups (product_id, operator_id, operator_markup_idr, agent_markup_idr)
SELECT id, operator_id,
       price_idr * operator_margin_bps / 10000,
       price_idr * agent_margin_bps / 10000
FROM products
ON CONFLICT (product_id, operator_id) DO NOTHING;

-- The order records which price it was charged at, and what each level's
-- contribution to that price was. Frozen at sale time like the split it
-- replaces, so a later markup change never rewrites a past order — and
-- itemised, so a disputed charge can be taken apart months later without
-- guessing what the settings were that day.
ALTER TABLE orders
  ADD COLUMN buyer_kind TEXT NOT NULL DEFAULT 'PILGRIM'
    CHECK (buyer_kind IN ('PILGRIM','AGENT')),
  ADD COLUMN base_price_idr BIGINT NOT NULL DEFAULT 0 CHECK (base_price_idr >= 0),
  ADD COLUMN operator_markup_idr BIGINT NOT NULL DEFAULT 0 CHECK (operator_markup_idr >= 0),
  ADD COLUMN agent_markup_idr BIGINT NOT NULL DEFAULT 0 CHECK (agent_markup_idr >= 0);

-- buyer_kind must agree with which buyer column is filled. Two sources for one
-- fact is a bug waiting to happen; this makes them unable to disagree.
ALTER TABLE orders ADD CONSTRAINT orders_buyer_kind_matches_buyer_check
  CHECK (
    (buyer_kind = 'PILGRIM' AND pilgrim_id IS NOT NULL AND buyer_agent_id IS NULL) OR
    (buyer_kind = 'AGENT'   AND buyer_agent_id IS NOT NULL AND pilgrim_id IS NULL)
  );

-- An agent's price contains no agent markup, by definition of what the agent
-- price is. Stated as a constraint because it is the rule that decides what an
-- agent is owed, and it should not depend on every call site remembering it.
ALTER TABLE orders ADD CONSTRAINT orders_agent_buyer_pays_no_agent_markup_check
  CHECK (buyer_kind <> 'AGENT' OR agent_markup_idr = 0);

-- +goose Down
ALTER TABLE orders
  DROP CONSTRAINT IF EXISTS orders_agent_buyer_pays_no_agent_markup_check,
  DROP CONSTRAINT IF EXISTS orders_buyer_kind_matches_buyer_check,
  DROP COLUMN IF EXISTS agent_markup_idr,
  DROP COLUMN IF EXISTS operator_markup_idr,
  DROP COLUMN IF EXISTS base_price_idr,
  DROP COLUMN IF EXISTS buyer_kind;

DROP TABLE IF EXISTS product_markups;
ALTER TABLE products DROP COLUMN IF EXISTS base_price_idr;
