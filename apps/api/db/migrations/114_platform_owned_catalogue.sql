-- +goose Up

-- Digital products are supplied by TawafiqHub. Only the platform holds the
-- supplier contracts, the API routes and the credentials — a travel neither
-- sets nor sees any of it (owner's rule). But the catalogue was still owned
-- per tenant: every travel that wanted to sell pulsa needed its own products
-- row, and the platform had to configure routing separately for each copy of
-- what is physically the same product.
--
-- So a NULL operator_id now means the platform owns it. NULL rather than a
-- sentinel operator row: a fake tenant would appear in every count, every
-- listing and every join that walks operators, and one forgotten filter would
-- show it to a customer.
--
-- The rule that makes this safe, and it is a rule about code as much as
-- schema: READS WIDEN, WRITES NEVER.
--
--   selling reads   operator_id = $1 OR operator_id IS NULL
--   tenant writes   operator_id = $1
--
-- Forgetting to widen a read hides a platform product — visible, annoying,
-- harmless. Widening a write would let a travel edit the catalogue every other
-- travel sells from. Only one of those is recoverable, so the unsafe direction
-- is the one no query is allowed to take.

ALTER TABLE products
  ALTER COLUMN operator_id DROP NOT NULL,
  -- A digital product is not seasonal. Pulsa does not belong to Umrah 2026.
  ALTER COLUMN season_id DROP NOT NULL;

-- A platform product must have no season, and a tenant product must have one.
-- Half-migrated rows are the thing that makes a change like this unsafe to
-- reason about later, so the two states are made unable to blur.
ALTER TABLE products ADD CONSTRAINT products_ownership_is_consistent_check
  CHECK ((operator_id IS NULL) = (season_id IS NULL));

-- Codes were unique per operator. Postgres treats NULLs as distinct in a
-- unique index, so with operator_id NULL that index stops constraining
-- anything and two platform products could share a code — the one thing a code
-- exists to prevent.
CREATE UNIQUE INDEX products_platform_code_idx
  ON products (code) WHERE operator_id IS NULL AND code <> '';

-- Only the platform supplies digital products, so only the platform may own
-- one. A travel creating its own PPOB row would be selling something it has no
-- route for, and the checkout gate would refuse it after the fact — better to
-- make the row impossible than to explain the refusal.
--
-- NOT VALID: existing per-tenant digital products are grandfathered rather
-- than deleted. Orders point at them and transaction records must never be
-- lost. New rows are checked from now on; the old ones are migrated by hand
-- once their orders have aged out.
ALTER TABLE products ADD CONSTRAINT products_digital_is_platform_owned_check
  CHECK (category NOT IN ('ROAMING_DATA','PPOB_CREDIT') OR operator_id IS NULL)
  NOT VALID;

-- Markups were already keyed by (product_id, operator_id), which is what makes
-- a shared product work: one catalogue row, one markup row per travel. Nothing
-- to change there — but the index that finds a travel's markups is worth
-- having now that one product carries many.
CREATE INDEX product_markups_operator_idx ON product_markups (operator_id);

-- Listings scoped to a season have to reach platform products too, and those
-- carry no season. Without this the widened read scans the whole table.
CREATE INDEX products_platform_active_idx
  ON products (category, is_active) WHERE operator_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS products_platform_active_idx;
DROP INDEX IF EXISTS product_markups_operator_idx;
DROP INDEX IF EXISTS products_platform_code_idx;
ALTER TABLE products
  DROP CONSTRAINT IF EXISTS products_digital_is_platform_owned_check,
  DROP CONSTRAINT IF EXISTS products_ownership_is_consistent_check;

-- Platform-owned rows cannot exist under the old NOT NULL columns. Removing
-- them is the only way back, and it is why this direction is a rollback for a
-- failed deploy rather than a routine reversal.
DELETE FROM products WHERE operator_id IS NULL;
ALTER TABLE products
  ALTER COLUMN operator_id SET NOT NULL,
  ALTER COLUMN season_id SET NOT NULL;
