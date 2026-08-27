-- +goose Up

-- Two changes that share one column, so they share one migration.
--
-- 1. An order had to belong to a pilgrim. But agents buy on their own account
--    too — jamaah routinely pay them in cash and the agent transacts on their
--    behalf — and there was nowhere to record that. The workaround in the
--    field would be to invent a fake pilgrim row, which pollutes the pilgrim
--    register and quietly inflates every count taken from it.
--
-- 2. pilgrim_id was ON DELETE CASCADE, so deleting a pilgrim destroyed their
--    orders: the money records, the split, the receipt number, the
--    destination number the transaction was sent to. Nothing in the
--    application deletes a pilgrim today, so nothing has been lost. But an
--    operator-level cascade or a manual cleanup would go straight through it
--    without a word, and transaction records are the one thing here that must
--    survive everything else.

-- An order now belongs to exactly one buyer: a pilgrim, or an agent.
ALTER TABLE orders ALTER COLUMN pilgrim_id DROP NOT NULL;

ALTER TABLE orders
  ADD COLUMN buyer_agent_id UUID REFERENCES agents(id) ON DELETE RESTRICT;

-- Exactly one, never both, never neither. Without this an order could end up
-- with no buyer at all, which reads as "unattributed money" — the state that
-- is hardest to unwind months later.
ALTER TABLE orders ADD CONSTRAINT orders_exactly_one_buyer_check
  CHECK ((pilgrim_id IS NULL) <> (buyer_agent_id IS NULL));

-- An agent buying for themselves earns no commission: they already hold the
-- agent price, which is the platform base plus the operator's markup and
-- nothing more. Paying commission on top would be paying them twice for the
-- same transaction, out of a margin that was never collected.
--
-- Enforced here rather than only in Go, because this is the rule that decides
-- whether money is owed, and code paths multiply.
ALTER TABLE orders ADD CONSTRAINT orders_agent_buyer_earns_no_commission_check
  CHECK (buyer_agent_id IS NULL OR agent_commission_idr = 0);

CREATE INDEX orders_buyer_agent_idx ON orders(buyer_agent_id)
  WHERE buyer_agent_id IS NOT NULL;

-- The cascade, replaced. RESTRICT refuses the delete instead of silently
-- taking the orders with it — a pilgrim with transaction history cannot be
-- erased, which is the intended answer.
--
-- Left deliberately alone for now: orders.operator_id and orders.season_id are
-- still ON DELETE CASCADE. Removing a whole tenant is a different decision
-- from removing one person, and it needs an archival destination before the
-- delete can be refused outright. Recorded in HANDOFF.
ALTER TABLE orders DROP CONSTRAINT orders_pilgrim_id_fkey;
ALTER TABLE orders
  ADD CONSTRAINT orders_pilgrim_id_fkey
  FOREIGN KEY (pilgrim_id) REFERENCES pilgrims(id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE orders DROP CONSTRAINT orders_pilgrim_id_fkey;
ALTER TABLE orders
  ADD CONSTRAINT orders_pilgrim_id_fkey
  FOREIGN KEY (pilgrim_id) REFERENCES pilgrims(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS orders_buyer_agent_idx;
ALTER TABLE orders
  DROP CONSTRAINT IF EXISTS orders_agent_buyer_earns_no_commission_check,
  DROP CONSTRAINT IF EXISTS orders_exactly_one_buyer_check,
  DROP COLUMN IF EXISTS buyer_agent_id;

-- Only safe because the Up direction is the only thing that can have created
-- agent-bought orders, and Down removes them with the column they depend on.
DELETE FROM orders WHERE pilgrim_id IS NULL;
ALTER TABLE orders ALTER COLUMN pilgrim_id SET NOT NULL;
