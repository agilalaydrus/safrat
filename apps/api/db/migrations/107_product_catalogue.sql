-- +goose Up

-- A product had a name and a price and nothing else to identify it by. Three
-- separate things were missing, and conflating them is how a digital product
-- catalogue goes wrong:
--
--   code    — what a person quotes. "PULSA-TSEL-10K", not a UUID.
--   nominal — the face value the customer receives. Pulsa of 10,000.
--   price   — what we charge for it. 11,500.
--
-- Plus supplier_cost_idr, already here, which is what we pay for it. Four
-- distinct numbers, and until now only one of them existed. Without nominal,
-- "10,000 pulsa sold for 11,500" and "11,500 pulsa sold at cost" are the same
-- row, and nobody can tell a margin from a mistake.
ALTER TABLE products
  ADD COLUMN code TEXT NOT NULL DEFAULT '',
  -- Nullable rather than zero: a travel package has no face value, and a
  -- nominal of zero would claim it has one worth nothing.
  ADD COLUMN nominal_idr BIGINT CHECK (nominal_idr IS NULL OR nominal_idr > 0);

-- Backfilled from the name so existing products are quotable immediately, and
-- so the uniqueness below has something distinct to work with. Uppercased and
-- stripped to the characters a code should have; collisions get the row's own
-- id appended, which is ugly and correct — an operator can rename it, and until
-- they do it is still unambiguous.
UPDATE products SET code =
  UPPER(REGEXP_REPLACE(LEFT(name, 24), '[^a-zA-Z0-9]+', '-', 'g')) || '-' || LEFT(id::text, 4)
WHERE code = '';

-- Unique per operator, not globally: two travels may both sell "PULSA-10K" and
-- neither should have to know the other exists.
CREATE UNIQUE INDEX products_operator_code_idx ON products (operator_id, code) WHERE code <> '';

-- +goose Down
DROP INDEX products_operator_code_idx;
ALTER TABLE products DROP COLUMN nominal_idr, DROP COLUMN code;
