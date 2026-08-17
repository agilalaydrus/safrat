-- +goose Up
-- A ledger, not a flag — an operator may pay an agent out in several
-- installments over time, and a single "paid" boolean on the agent (or on
-- each order) can't represent that. Outstanding balance is always derived
-- (sum of PAID orders' agent_commission_idr minus sum of this table), never
-- stored, so it can never drift out of sync with the orders it's based on.
CREATE TABLE agent_payouts (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  agent_id        UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  amount_idr      BIGINT      NOT NULL CHECK (amount_idr > 0),
  note            TEXT        NOT NULL DEFAULT '',
  paid_by_user_id TEXT        NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX agent_payouts_agent_idx ON agent_payouts(agent_id);
CREATE INDEX agent_payouts_operator_idx ON agent_payouts(operator_id);

-- +goose Down
DROP TABLE IF EXISTS agent_payouts;
