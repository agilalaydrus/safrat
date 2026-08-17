-- +goose Up
-- Leader-initiated withdrawal requests — the queue an operator works from,
-- distinct from agent_payouts (which is money that has actually moved).
-- A request never becomes a payout by mutation; RecordAgentPayout inserts a
-- fresh agent_payouts row and marks the request APPROVED, so the ledger
-- (agent_payouts) stays an append-only record of real disbursements even
-- when driven by a request.
CREATE TABLE agent_payout_requests (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id      UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  agent_id         UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  amount_idr       BIGINT      NOT NULL CHECK (amount_idr > 0),
  note             TEXT        NOT NULL DEFAULT '',
  status           TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
  resolution_note  TEXT        NOT NULL DEFAULT '',
  resolved_at      TIMESTAMPTZ,
  resolved_by_user_id TEXT,
  requested_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX agent_payout_requests_agent_idx ON agent_payout_requests(agent_id);
CREATE INDEX agent_payout_requests_operator_status_idx ON agent_payout_requests(operator_id, status);

ALTER TABLE agent_payouts
  ADD COLUMN request_id UUID REFERENCES agent_payout_requests(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE agent_payouts DROP COLUMN IF EXISTS request_id;
DROP TABLE IF EXISTS agent_payout_requests;
