-- +goose Up

-- The webhook marked an order PAID on the gateway's word that *something* was
-- paid, never checking that the amount matched what was owed. Commission,
-- revenue and the jamaah's payment history all followed from that word.
--
-- Commission counts only when the amount paid matches the price (owner's
-- rule), so the amount has to be checked, and a mismatch needs somewhere to go
-- that is neither "settled" nor "failed": money did arrive, and discarding the
-- transaction would strand it.
ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED','CANCELLED','REFUNDED','HELD'));

-- What the gateway said was actually paid, kept whether it matched or not —
-- for a held transaction it is the evidence, and for a settled one it is the
-- proof that the check was made.
ALTER TABLE orders ADD COLUMN paid_amount_idr BIGINT;
ALTER TABLE orders ADD COLUMN held_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX orders_held_idx ON orders (operator_id, created_at DESC) WHERE status = 'HELD';

-- +goose StatementBegin
-- A held transaction is still counted, because it neither failed nor was
-- refunded — it is waiting on a human. It is not settled, so it cannot be paid
-- out. Rebuilt rather than patched so the two definitions stay in one place.
CREATE OR REPLACE VIEW agent_commission_state AS
SELECT
  e.agent_id,
  e.operator_id,
  COALESCE(SUM(e.amount_idr), 0)::bigint AS recognised_idr,
  COALESCE(SUM(e.amount_idr) FILTER (
    WHERE e.order_id IS NULL OR o.status = 'PAID'
  ), 0)::bigint AS settled_idr,
  COALESCE(SUM(e.amount_idr) FILTER (
    WHERE o.status IN ('PENDING', 'HELD')
  ), 0)::bigint AS pending_idr
FROM agent_commission_entries e
LEFT JOIN orders o ON o.id = e.order_id
GROUP BY e.agent_id, e.operator_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE VIEW agent_commission_state AS
SELECT
  e.agent_id,
  e.operator_id,
  COALESCE(SUM(e.amount_idr), 0)::bigint AS recognised_idr,
  COALESCE(SUM(e.amount_idr) FILTER (
    WHERE e.order_id IS NULL OR o.status = 'PAID'
  ), 0)::bigint AS settled_idr,
  COALESCE(SUM(e.amount_idr) FILTER (
    WHERE o.status = 'PENDING'
  ), 0)::bigint AS pending_idr
FROM agent_commission_entries e
LEFT JOIN orders o ON o.id = e.order_id
GROUP BY e.agent_id, e.operator_id;
-- +goose StatementEnd
DROP INDEX orders_held_idx;
ALTER TABLE orders DROP COLUMN held_reason;
ALTER TABLE orders DROP COLUMN paid_amount_idr;
ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED','CANCELLED','REFUNDED'));
