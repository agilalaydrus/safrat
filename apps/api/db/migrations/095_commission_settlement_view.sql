-- +goose Up

-- Commission is recognised the moment a transaction is placed, not when it
-- settles: a pending transaction counts towards everything related to it,
-- and only failure or a refund takes it back (owner's rule).
--
-- That creates two different questions with two different answers, and
-- conflating them would let an agent withdraw money for a transaction nobody
-- has paid for yet:
--
--   recognised — everything earned, pending included. What the agent has made.
--   settled    — only what sits behind a transaction that actually completed.
--                The only money that may be paid out.
--
-- Neither is a stored figure. Both are the ledger read two ways, so they can
-- never drift apart from each other or from the transactions underneath them.
CREATE VIEW agent_commission_state AS
SELECT
  e.agent_id,
  e.operator_id,
  COALESCE(SUM(e.amount_idr), 0)::bigint AS recognised_idr,
  -- An entry with no order is a manual adjustment: there is no transaction to
  -- wait on, so it is settled by definition.
  COALESCE(SUM(e.amount_idr) FILTER (
    WHERE e.order_id IS NULL OR o.status = 'PAID'
  ), 0)::bigint AS settled_idr,
  COALESCE(SUM(e.amount_idr) FILTER (
    WHERE o.status = 'PENDING'
  ), 0)::bigint AS pending_idr
FROM agent_commission_entries e
LEFT JOIN orders o ON o.id = e.order_id
GROUP BY e.agent_id, e.operator_id;

-- +goose Down
DROP VIEW agent_commission_state;
