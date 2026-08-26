-- +goose Up

-- A refund always returns the whole transaction. Partial refunds are not a
-- feature that is discouraged; they are not a thing that can exist.
--
-- The RPC no longer carries an amount at all, so the service cannot express a
-- partial refund. These are the guarantees underneath that: a path which never
-- goes through the service — a script, a fixture, a future endpoint — still
-- cannot create one.

-- One refund per order, ever. With the amount pinned to the order's total
-- below, this is also what makes "refunded twice" impossible.
CREATE UNIQUE INDEX order_refunds_one_per_order_idx ON order_refunds (order_id);

-- +goose StatementBegin
-- Replaces the running-total check from migration 092: with refunds whole and
-- unique, the rule is simply that the amount equals what was paid.
CREATE OR REPLACE FUNCTION order_refund_within_amount_paid() RETURNS trigger AS $$
DECLARE
  order_status TEXT;
  order_total  BIGINT;
BEGIN
  SELECT status, total_price_idr INTO order_status, order_total
  FROM orders WHERE id = NEW.order_id FOR UPDATE;

  IF order_status NOT IN ('PAID', 'REFUNDED') THEN
    RAISE EXCEPTION 'cannot refund order % with status %', NEW.order_id, order_status
      USING ERRCODE = 'check_violation';
  END IF;

  IF NEW.amount_idr <> order_total THEN
    RAISE EXCEPTION 'refund for order % is %, but a refund must return the full % paid',
      NEW.order_id, NEW.amount_idr, order_total
      USING ERRCODE = 'check_violation';
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Migration 091 loosened these two indexes so that a partial refund could
-- reverse commission and credit a balance more than once per order. That
-- reason is gone, so the stricter rule comes back: exactly one earning and at
-- most one reversal per order, one purchase and at most one refund per pilgrim
-- balance. ADJUSTMENT entries stay unconstrained — a manual correction is not
-- a transaction event and may legitimately happen more than once.
DROP INDEX agent_commission_entries_order_earned_idx;
CREATE UNIQUE INDEX agent_commission_entries_order_kind_idx
  ON agent_commission_entries (order_id, kind)
  WHERE order_id IS NOT NULL AND kind IN ('EARNED', 'REVERSED');

DROP INDEX pilgrim_balance_entries_order_purchase_idx;
CREATE UNIQUE INDEX pilgrim_balance_entries_order_kind_idx
  ON pilgrim_balance_entries (order_id, kind)
  WHERE order_id IS NOT NULL AND kind IN ('PURCHASE', 'REFUND');

-- +goose Down
DROP INDEX pilgrim_balance_entries_order_kind_idx;
CREATE UNIQUE INDEX pilgrim_balance_entries_order_purchase_idx
  ON pilgrim_balance_entries (order_id) WHERE order_id IS NOT NULL AND kind = 'PURCHASE';
DROP INDEX agent_commission_entries_order_kind_idx;
CREATE UNIQUE INDEX agent_commission_entries_order_earned_idx
  ON agent_commission_entries (order_id) WHERE order_id IS NOT NULL AND kind = 'EARNED';
DROP INDEX order_refunds_one_per_order_idx;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION order_refund_within_amount_paid() RETURNS trigger AS $$
DECLARE
  order_status TEXT;
  order_total  BIGINT;
  refunded     BIGINT;
BEGIN
  SELECT status, total_price_idr INTO order_status, order_total
  FROM orders WHERE id = NEW.order_id FOR UPDATE;

  IF order_status NOT IN ('PAID', 'REFUNDED') THEN
    RAISE EXCEPTION 'cannot refund order % with status %', NEW.order_id, order_status
      USING ERRCODE = 'check_violation';
  END IF;

  SELECT COALESCE(SUM(amount_idr), 0) INTO refunded
  FROM order_refunds WHERE order_id = NEW.order_id;

  IF refunded > order_total THEN
    RAISE EXCEPTION 'refunds for order % total %, which exceeds the % paid',
      NEW.order_id, refunded, order_total
      USING ERRCODE = 'check_violation';
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
