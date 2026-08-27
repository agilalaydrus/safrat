-- +goose Up

-- The provider destination belongs to the transaction, not to the buyer's
-- mutable profile. Without this, changing a phone number after checkout
-- rewrites what a historical purchase appears to have targeted; an agent
-- buyer also has no pilgrim row for the old runtime join to read from.
ALTER TABLE orders ADD COLUMN destination TEXT NOT NULL DEFAULT '';

-- Preserve the value the legacy dispatch path would have used for existing
-- orders. Agent-bought orders cannot predate migration 110 in normal code, but
-- the second branch keeps the backfill honest for manually inserted records.
UPDATE orders o
SET destination = COALESCE(
  (SELECT NULLIF(p.phone, '') FROM pilgrims p WHERE p.id = o.pilgrim_id),
  (SELECT NULLIF(a.phone, '') FROM agents a WHERE a.id = o.buyer_agent_id),
  ''
)
WHERE EXISTS (
  SELECT 1 FROM products pr
  WHERE pr.id = o.product_id
  AND pr.category IN ('ROAMING_DATA', 'PPOB_CREDIT')
)
AND o.destination = '';

COMMENT ON COLUMN orders.destination IS
  'Phone/account supplied to the provider, frozen at order creation; empty only where the product has no digital destination.';

-- +goose Down
ALTER TABLE orders DROP COLUMN IF EXISTS destination;
