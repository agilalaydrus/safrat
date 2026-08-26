-- +goose Up
-- Append-only ledgers for the two balances the business actually owes people:
-- an agent's earned commission, and a pilgrim's deposit.
--
-- Neither existed. Commission was a column on orders (agent_commission_idr)
-- summed over PAID rows, so "reversing" it meant changing an order's status and
-- watching the number silently move — nothing recorded that a reversal had
-- happened, who did it, or why. A pilgrim balance did not exist at all, so a
-- refund had nowhere to go.
--
-- Append-only is the point. A reversal is a new negative entry, never an edit
-- of the original: an edited history cannot be audited, and these are exactly
-- the records someone will dispute.

CREATE TABLE agent_commission_entries (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- RESTRICT, not CASCADE: an agent with commission history is deactivated
  -- (agents.is_active), never deleted — removing them would erase the record of
  -- money they earned.
  agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
  -- Signed: EARNED is positive, REVERSED negative. The balance is the sum, so
  -- there is no separate "current balance" that can drift out of step.
  amount_idr      BIGINT NOT NULL CHECK (amount_idr <> 0),
  kind            VARCHAR(16) NOT NULL CHECK (kind IN ('EARNED', 'REVERSED', 'ADJUSTMENT')),
  -- What caused this entry, so any figure can be traced back to its cause.
  order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
  note            TEXT NOT NULL DEFAULT '',
  created_by_user_id TEXT,
  -- Same key means an advice about the same entry, never a new one.
  idempotency_key TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX agent_commission_entries_agent_idx ON agent_commission_entries (agent_id, created_at DESC);
CREATE UNIQUE INDEX agent_commission_entries_idempotency_idx
  ON agent_commission_entries (agent_id, idempotency_key) WHERE idempotency_key <> '';
-- An order can be earned once and reversed once, never twice either way.
CREATE UNIQUE INDEX agent_commission_entries_order_kind_idx
  ON agent_commission_entries (order_id, kind) WHERE order_id IS NOT NULL;

CREATE TABLE pilgrim_balance_entries (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id      UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  amount_idr      BIGINT NOT NULL CHECK (amount_idr <> 0),
  kind            VARCHAR(16) NOT NULL CHECK (kind IN ('DEPOSIT', 'REFUND', 'PURCHASE', 'WITHDRAWAL', 'ADJUSTMENT')),
  order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
  note            TEXT NOT NULL DEFAULT '',
  created_by_user_id TEXT,
  idempotency_key TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX pilgrim_balance_entries_pilgrim_idx ON pilgrim_balance_entries (pilgrim_id, created_at DESC);
CREATE UNIQUE INDEX pilgrim_balance_entries_idempotency_idx
  ON pilgrim_balance_entries (pilgrim_id, idempotency_key) WHERE idempotency_key <> '';
CREATE UNIQUE INDEX pilgrim_balance_entries_order_kind_idx
  ON pilgrim_balance_entries (order_id, kind) WHERE order_id IS NOT NULL;

-- +goose StatementBegin
-- Enforced, not merely intended: a ledger that can be edited is not a ledger,
-- and the guarantee has to survive someone reaching for psql at 2am.
--
-- UPDATE is refused outright — altering a recorded amount is exactly the thing
-- an audit trail exists to prevent. DELETE is refused too, but can be permitted
-- for one transaction by setting app.allow_ledger_purge, which tearing down a
-- whole tenant legitimately needs. Requiring that to be stated explicitly keeps
-- ordinary application code, and an ordinary mistake, from removing history.
CREATE OR REPLACE FUNCTION ledger_is_append_only() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' AND current_setting('app.allow_ledger_purge', true) = 'on' THEN
    RETURN OLD;
  END IF;
  RAISE EXCEPTION 'ledger entries are append-only; record a reversing entry instead of % on %', TG_OP, TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER agent_commission_entries_append_only
  BEFORE UPDATE OR DELETE ON agent_commission_entries
  FOR EACH ROW EXECUTE FUNCTION ledger_is_append_only();
CREATE TRIGGER pilgrim_balance_entries_append_only
  BEFORE UPDATE OR DELETE ON pilgrim_balance_entries
  FOR EACH ROW EXECUTE FUNCTION ledger_is_append_only();

-- Backfill what agents have already earned. Without this every agent's
-- outstanding balance would drop to zero the moment the reader switches to the
-- ledger — money they are genuinely owed, erased by a migration.
INSERT INTO agent_commission_entries (operator_id, agent_id, amount_idr, kind, order_id, note)
SELECT o.operator_id, o.agent_id, o.agent_commission_idr, 'EARNED', o.id,
       'Backfill dari pesanan lunas sebelum ledger komisi ada'
FROM orders o
WHERE o.status = 'PAID' AND o.agent_id IS NOT NULL AND o.agent_commission_idr > 0;

-- +goose Down
DROP TRIGGER IF EXISTS pilgrim_balance_entries_append_only ON pilgrim_balance_entries;
DROP TRIGGER IF EXISTS agent_commission_entries_append_only ON agent_commission_entries;
DROP FUNCTION IF EXISTS ledger_is_append_only();
DROP TABLE IF EXISTS pilgrim_balance_entries;
DROP TABLE IF EXISTS agent_commission_entries;
