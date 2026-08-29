-- +goose Up

-- A checkout attempt is distinct from an order's eventual payment outcome.
-- Keep the lane explicit so operator-recorded cash/bank sales are never
-- mistaken for repeated gateway attempts by the buyer.
ALTER TABLE orders
  ADD COLUMN checkout_channel VARCHAR(16) NOT NULL DEFAULT 'LEGACY',
  ADD COLUMN risk_level VARCHAR(12) NOT NULL DEFAULT 'NORMAL',
  ADD COLUMN risk_reason TEXT NOT NULL DEFAULT '';

ALTER TABLE orders
  ADD CONSTRAINT orders_checkout_channel_check
    CHECK (checkout_channel IN ('LEGACY', 'XENDIT', 'MANUAL')),
  ADD CONSTRAINT orders_risk_level_check
    CHECK (risk_level IN ('NORMAL', 'REVIEW')),
  ADD CONSTRAINT orders_risk_reason_check
    CHECK ((risk_level = 'NORMAL' AND risk_reason = '')
      OR (risk_level = 'REVIEW' AND risk_reason <> ''));

-- Preserve the only gateway evidence legacy rows have. Rows without an
-- invoice cannot be classified reliably and remain LEGACY rather than being
-- guessed into either the manual or gateway lane.
UPDATE orders SET checkout_channel = 'XENDIT'
WHERE xendit_invoice_id IS NOT NULL AND xendit_invoice_id <> '';

CREATE INDEX orders_gateway_attempt_buyer_idx
  ON orders (buyer_kind, (COALESCE(pilgrim_id, buyer_agent_id)), created_at DESC)
  WHERE checkout_channel = 'XENDIT';

CREATE INDEX orders_review_risk_idx
  ON orders (operator_id, created_at DESC)
  WHERE risk_level = 'REVIEW';

-- Checkout policy, enforced under a per-buyer advisory lock:
--   * replays of an existing idempotency key do not consume another attempt;
--   * the fourth and fifth new gateway checkout in one rolling hour are
--     accepted but marked for human review if money arrives;
--   * the sixth is refused;
--   * a buyer with unresolved held money cannot open another gateway checkout.
--
-- The lock is the invariant. Counting then inserting without it lets six
-- concurrent requests all observe zero and all pass.
-- +goose StatementBegin
CREATE FUNCTION orders_fraud_guard() RETURNS trigger AS $$
DECLARE
  owner_id UUID;
  recent_attempts INTEGER;
BEGIN
  IF TG_OP = 'UPDATE' THEN
    IF NEW.checkout_channel <> OLD.checkout_channel
       OR NEW.risk_level <> OLD.risk_level
       OR NEW.risk_reason <> OLD.risk_reason THEN
      RAISE EXCEPTION 'order fraud evidence is immutable';
    END IF;
    IF OLD.status = 'PENDING' AND NEW.status = 'PAID'
       AND OLD.risk_level = 'REVIEW' THEN
      RAISE EXCEPTION 'flagged order must enter HELD before it can be settled'
        USING ERRCODE = 'check_violation',
              CONSTRAINT = 'orders_flagged_payment_requires_hold';
    END IF;
    RETURN NEW;
  END IF;

  IF NEW.checkout_channel <> 'XENDIT' THEN
    RETURN NEW;
  END IF;

  owner_id := COALESCE(NEW.pilgrim_id, NEW.buyer_agent_id);
  PERFORM pg_advisory_xact_lock(
    hashtextextended('checkout:' || NEW.buyer_kind || ':' || owner_id::text, 0));

  -- ON CONFLICT happens after BEFORE INSERT triggers. Recognise a retry here
  -- first so a replay is not refused by limits created by its own first call.
  IF NEW.idempotency_key <> '' AND EXISTS (
    SELECT 1 FROM orders
    WHERE operator_id = NEW.operator_id
      AND idempotency_key = NEW.idempotency_key
  ) THEN
    RETURN NEW;
  END IF;

  IF EXISTS (
    SELECT 1 FROM orders
    WHERE buyer_kind = NEW.buyer_kind
      AND COALESCE(pilgrim_id, buyer_agent_id) = owner_id
      AND status = 'HELD'
  ) THEN
    RAISE EXCEPTION 'buyer has an unresolved held payment'
      USING ERRCODE = 'check_violation',
            CONSTRAINT = 'orders_checkout_held_block';
  END IF;

  SELECT COUNT(*)::integer INTO recent_attempts
  FROM orders
  WHERE checkout_channel = 'XENDIT'
    AND buyer_kind = NEW.buyer_kind
    AND COALESCE(pilgrim_id, buyer_agent_id) = owner_id
    AND created_at > NOW() - INTERVAL '1 hour';

  IF recent_attempts >= 5 THEN
    RAISE EXCEPTION 'buyer exceeded five gateway checkout attempts in one hour'
      USING ERRCODE = 'check_violation',
            CONSTRAINT = 'orders_checkout_attempt_limit';
  END IF;

  IF recent_attempts >= 3 THEN
    NEW.risk_level := 'REVIEW';
    NEW.risk_reason := format(
      'Checkout gateway ke-%s dalam 1 jam; pembayaran wajib ditinjau',
      recent_attempts + 1);
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER orders_fraud_guard_trigger
  BEFORE INSERT OR UPDATE ON orders
  FOR EACH ROW EXECUTE FUNCTION orders_fraud_guard();

-- +goose Down

DROP TRIGGER orders_fraud_guard_trigger ON orders;
DROP FUNCTION orders_fraud_guard();
DROP INDEX orders_review_risk_idx;
DROP INDEX orders_gateway_attempt_buyer_idx;
ALTER TABLE orders
  DROP CONSTRAINT orders_risk_reason_check,
  DROP CONSTRAINT orders_risk_level_check,
  DROP CONSTRAINT orders_checkout_channel_check,
  DROP COLUMN risk_reason,
  DROP COLUMN risk_level,
  DROP COLUMN checkout_channel;
