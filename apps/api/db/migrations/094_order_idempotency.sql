-- +goose Up

-- Creating an order had no idempotency key at all. A double-tapped checkout
-- button made two orders and two Xendit invoices, and a jamaah could pay both
-- — charged twice for one intent, with no way for the system to tell that the
-- second request was the same intent as the first.
--
-- Scoped per operator rather than globally: the key is minted by a client, and
-- two tenants must never be able to collide with, or probe for, each other's.
ALTER TABLE orders ADD COLUMN idempotency_key TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX orders_idempotency_idx
  ON orders (operator_id, idempotency_key) WHERE idempotency_key <> '';

-- Who actually placed the order, when it was not the jamaah themselves. The
-- commission still follows the referral (orders.agent_id comes from
-- pilgrims.agent_id), so this answers "who sold it", never "who earns from it"
-- — keeping the two apart is what stops a seller from taking a commission that
-- belongs to the referrer.
ALTER TABLE orders ADD COLUMN placed_by_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL;
CREATE INDEX orders_placed_by_agent_idx ON orders (placed_by_agent_id) WHERE placed_by_agent_id IS NOT NULL;

-- +goose Down
DROP INDEX orders_placed_by_agent_idx;
ALTER TABLE orders DROP COLUMN placed_by_agent_id;
DROP INDEX orders_idempotency_idx;
ALTER TABLE orders DROP COLUMN idempotency_key;
