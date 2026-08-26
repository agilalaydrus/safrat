-- +goose Up

-- A refunded order used to have nowhere to go: status allowed only
-- PENDING/PAID/EXPIRED/FAILED/CANCELLED, so an order whose money went back to
-- the pilgrim stayed PAID forever. Revenue was overstated and the agent stayed
-- credited for a sale that was undone.
ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED','CANCELLED','REFUNDED'));

-- Commission may be reversed more than once, because a refund may be partial.
-- The "one entry per order" guarantee still matters for the earning itself —
-- that is what stops a redelivered payment webhook paying twice — so the index
-- is narrowed to EARNED rather than dropped.
DROP INDEX agent_commission_entries_order_kind_idx;
CREATE UNIQUE INDEX agent_commission_entries_order_earned_idx
  ON agent_commission_entries (order_id) WHERE order_id IS NOT NULL AND kind = 'EARNED';

-- The same reasoning applies to a pilgrim's balance: one PURCHASE per order,
-- but a partial refund may credit the balance more than once.
DROP INDEX pilgrim_balance_entries_order_kind_idx;
CREATE UNIQUE INDEX pilgrim_balance_entries_order_purchase_idx
  ON pilgrim_balance_entries (order_id) WHERE order_id IS NOT NULL AND kind = 'PURCHASE';

-- One row per refund event. Migration 093 narrows this further: a refund
-- always returns the whole transaction, and an order can only have one.
CREATE TABLE order_refunds (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- RESTRICT: an order that has been refunded is a financial record. Deleting
  -- it would orphan the balance and commission entries that reference it.
  order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
  amount_idr BIGINT NOT NULL CHECK (amount_idr > 0),
  -- How much agent commission this refund clawed back. Stored rather than
  -- recomputed, so the arithmetic behind a historical reversal stays readable
  -- even if the order's commission is later corrected.
  commission_reversed_idr BIGINT NOT NULL DEFAULT 0 CHECK (commission_reversed_idr >= 0),
  reason TEXT NOT NULL DEFAULT '',
  -- Who approved it. There is no gateway confirmation behind a manual refund,
  -- so this is the accountability trail.
  created_by_user_id TEXT,
  idempotency_key TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX order_refunds_order_idx ON order_refunds (order_id, created_at DESC);
CREATE INDEX order_refunds_operator_idx ON order_refunds (operator_id, created_at DESC);

-- A replayed refund request settles the same refund rather than issuing a
-- second one. Enforced here, not by a SELECT-then-INSERT that two concurrent
-- requests would both pass.
CREATE UNIQUE INDEX order_refunds_idempotency_idx
  ON order_refunds (order_id, idempotency_key) WHERE idempotency_key <> '';

-- Refunds are history, exactly like the ledgers they drive.
CREATE TRIGGER order_refunds_append_only
  BEFORE UPDATE OR DELETE ON order_refunds
  FOR EACH ROW EXECUTE FUNCTION ledger_is_append_only();

-- +goose Down
DROP TABLE order_refunds;
DROP INDEX pilgrim_balance_entries_order_purchase_idx;
CREATE UNIQUE INDEX pilgrim_balance_entries_order_kind_idx
  ON pilgrim_balance_entries (order_id, kind) WHERE order_id IS NOT NULL;
DROP INDEX agent_commission_entries_order_earned_idx;
CREATE UNIQUE INDEX agent_commission_entries_order_kind_idx
  ON agent_commission_entries (order_id, kind) WHERE order_id IS NOT NULL;
ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED','CANCELLED'));
