-- +goose Up

-- Refunds broke a proxy that several queries relied on: "how much was paid"
-- was read as orders.total_price_idr filtered by status = 'PAID'. That was
-- true until money could go back. A partially refunded order stays PAID with
-- its full price, so every consumer overstated.
--
-- The worst of them was not a report. CancellationService computes
-- refundAmount = totalPaid * refundPct from GetPilgrimPaidTotal, so an
-- inflated "paid" figure made the operator refund a second time the portion
-- they had already returned.
--
-- Fixing the three call sites would have left the next query to make the same
-- mistake. This view is the single answer to the question instead, and it is
-- built so that using it wrongly is hard: net_paid_idr is already zero for
-- orders that were never paid, so a caller needs no status filter at all and
-- cannot forget one.
CREATE VIEW order_payments AS
SELECT
  o.id              AS order_id,
  o.operator_id,
  o.season_id,
  o.pilgrim_id,
  o.agent_id,
  o.product_id,
  o.status,
  o.total_price_idr,
  COALESCE(r.refunded_idr, 0)::bigint AS refunded_idr,
  -- What the operator still holds from this order today. A fully refunded
  -- order reaches zero by arithmetic, not by its status, so the two can never
  -- disagree.
  CASE
    WHEN o.status IN ('PAID', 'REFUNDED')
      THEN o.total_price_idr - COALESCE(r.refunded_idr, 0)
    ELSE 0
  END::bigint AS net_paid_idr,
  o.paid_at,
  o.created_at
FROM orders o
LEFT JOIN (
  SELECT order_id, SUM(amount_idr) AS refunded_idr
  FROM order_refunds
  GROUP BY order_id
) r ON r.order_id = o.id;

-- +goose StatementBegin
-- The service already checks this under a row lock, and that check is where a
-- caller gets a helpful message. This trigger is the backstop: it makes the
-- invariant true of the data itself, so no future code path — a script, a
-- migration, a new endpoint that forgets the lock — can refund more than was
-- paid or refund an order that never was.
CREATE OR REPLACE FUNCTION order_refund_within_amount_paid() RETURNS trigger AS $$
DECLARE
  order_status TEXT;
  order_total  BIGINT;
  refunded     BIGINT;
BEGIN
  -- FOR UPDATE, so two refunds inserted concurrently are serialised here even
  -- if the caller took no lock of its own: the second one's sum includes the
  -- first.
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

-- AFTER INSERT, so the row being inserted is already part of the sum.
CREATE TRIGGER order_refunds_within_amount_paid
  AFTER INSERT ON order_refunds
  FOR EACH ROW EXECUTE FUNCTION order_refund_within_amount_paid();

-- +goose Down
DROP TRIGGER order_refunds_within_amount_paid ON order_refunds;
DROP FUNCTION order_refund_within_amount_paid();
DROP VIEW order_payments;
