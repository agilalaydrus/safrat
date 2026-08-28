-- +goose Up

-- A purchase made by an agent is customer money, not commission. Keep its
-- refund in a dedicated append-only ledger so it can never inflate earnings.
CREATE TABLE agent_refund_balance_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
  amount_idr BIGINT NOT NULL CHECK (amount_idr <> 0),
  kind VARCHAR(16) NOT NULL CHECK (kind IN ('REFUND', 'WITHDRAWAL', 'ADJUSTMENT')),
  order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
  note TEXT NOT NULL DEFAULT '',
  created_by_user_id TEXT,
  idempotency_key TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX agent_refund_balance_agent_idx
  ON agent_refund_balance_entries (agent_id, created_at DESC);
CREATE UNIQUE INDEX agent_refund_balance_idempotency_idx
  ON agent_refund_balance_entries (agent_id, idempotency_key)
  WHERE idempotency_key <> '';
CREATE UNIQUE INDEX agent_refund_balance_order_refund_idx
  ON agent_refund_balance_entries (order_id)
  WHERE order_id IS NOT NULL AND kind = 'REFUND';
CREATE TRIGGER agent_refund_balance_append_only
  BEFORE UPDATE OR DELETE ON agent_refund_balance_entries
  FOR EACH ROW EXECUTE FUNCTION ledger_is_append_only();

-- Evolve the already-shipped pilgrim payout workflow without rewriting its
-- history. Existing rows become PILGRIM beneficiaries; new rows may instead
-- belong to the authenticated agent who made the original purchase.
DROP TRIGGER IF EXISTS paid_refund_payout_has_withdrawal_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS paid_refund_payout_has_withdrawal();
DROP TRIGGER IF EXISTS pilgrim_refund_payout_guard_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS pilgrim_refund_payout_guard();
DROP TRIGGER IF EXISTS pilgrim_refund_payout_balance_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS pilgrim_refund_payout_has_balance();

ALTER TABLE pilgrim_refund_payout_requests
  ALTER COLUMN pilgrim_id DROP NOT NULL,
  ADD COLUMN beneficiary_kind VARCHAR(16) NOT NULL DEFAULT 'PILGRIM',
  ADD COLUMN agent_id UUID REFERENCES agents(id) ON DELETE RESTRICT,
  ADD COLUMN destination_channel TEXT NOT NULL DEFAULT '',
  ADD COLUMN destination_account_holder TEXT NOT NULL DEFAULT '',
  ADD COLUMN destination_account_encrypted TEXT NOT NULL DEFAULT '',
  ADD COLUMN destination_account_last4 VARCHAR(4) NOT NULL DEFAULT '',
  ADD COLUMN provider VARCHAR(16) NOT NULL DEFAULT '',
  ADD COLUMN provider_payout_id TEXT,
  ADD COLUMN provider_status TEXT NOT NULL DEFAULT '',
  ADD COLUMN provider_failure_code TEXT NOT NULL DEFAULT '',
  ADD COLUMN provider_last_attempt_at TIMESTAMPTZ,
  ADD COLUMN proof_url TEXT NOT NULL DEFAULT '';

ALTER TABLE pilgrim_refund_payout_requests
  ADD CONSTRAINT refund_payout_beneficiary_kind_check
    CHECK (beneficiary_kind IN ('PILGRIM', 'AGENT')),
  ADD CONSTRAINT refund_payout_exactly_one_beneficiary_check
    CHECK (
      (beneficiary_kind = 'PILGRIM' AND pilgrim_id IS NOT NULL AND agent_id IS NULL)
      OR
      (beneficiary_kind = 'AGENT' AND agent_id IS NOT NULL AND pilgrim_id IS NULL)
    );

ALTER TABLE pilgrim_refund_payout_requests DROP CONSTRAINT IF EXISTS pilgrim_refund_payout_requests_pilgrim_id_idempotency_key_key;
CREATE UNIQUE INDEX refund_payout_pilgrim_idempotency_idx
  ON pilgrim_refund_payout_requests (pilgrim_id, idempotency_key)
  WHERE pilgrim_id IS NOT NULL;
CREATE UNIQUE INDEX refund_payout_agent_idempotency_idx
  ON pilgrim_refund_payout_requests (agent_id, idempotency_key)
  WHERE agent_id IS NOT NULL;
CREATE INDEX refund_payout_agent_idx
  ON pilgrim_refund_payout_requests (agent_id, created_at DESC)
  WHERE agent_id IS NOT NULL;
CREATE UNIQUE INDEX refund_payout_provider_id_idx
  ON pilgrim_refund_payout_requests (provider, provider_payout_id)
  WHERE provider <> '' AND provider_payout_id IS NOT NULL;

ALTER TABLE pilgrim_refund_payout_requests DROP CONSTRAINT pilgrim_refund_payout_requests_status_check;
ALTER TABLE pilgrim_refund_payout_requests ADD CONSTRAINT pilgrim_refund_payout_requests_status_check
  CHECK (status IN ('REQUESTED', 'PROCESSING', 'PAID', 'FAILED', 'REVERSED'));

-- +goose StatementBegin
CREATE FUNCTION refund_payout_has_balance() RETURNS trigger AS $$
DECLARE
  actual_operator UUID;
  current_balance BIGINT;
  already_reserved BIGINT;
  owner_key TEXT;
BEGIN
  owner_key := CASE WHEN NEW.beneficiary_kind = 'PILGRIM'
    THEN NEW.pilgrim_id::text ELSE NEW.agent_id::text END;
  PERFORM pg_advisory_xact_lock(hashtextextended(owner_key, 0));

  IF NEW.beneficiary_kind = 'PILGRIM' THEN
    SELECT operator_id INTO actual_operator FROM pilgrims WHERE id = NEW.pilgrim_id;
    SELECT COALESCE(SUM(amount_idr), 0)::bigint INTO current_balance
      FROM pilgrim_balance_entries WHERE pilgrim_id = NEW.pilgrim_id;
    SELECT COALESCE(SUM(amount_idr), 0)::bigint INTO already_reserved
      FROM pilgrim_refund_payout_requests
      WHERE pilgrim_id = NEW.pilgrim_id AND status IN ('REQUESTED', 'PROCESSING');
  ELSE
    SELECT operator_id INTO actual_operator FROM agents WHERE id = NEW.agent_id;
    SELECT COALESCE(SUM(amount_idr), 0)::bigint INTO current_balance
      FROM agent_refund_balance_entries WHERE agent_id = NEW.agent_id;
    SELECT COALESCE(SUM(amount_idr), 0)::bigint INTO already_reserved
      FROM pilgrim_refund_payout_requests
      WHERE agent_id = NEW.agent_id AND status IN ('REQUESTED', 'PROCESSING');
  END IF;

  IF actual_operator IS NULL OR actual_operator <> NEW.operator_id THEN
    RAISE EXCEPTION 'payout request operator does not own beneficiary %', owner_key
      USING ERRCODE = 'check_violation';
  END IF;
  IF NEW.amount_idr > current_balance - already_reserved THEN
    RAISE EXCEPTION 'refund payout % exceeds available balance % for beneficiary %',
      NEW.amount_idr, current_balance - already_reserved, owner_key
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER refund_payout_balance_trigger
  BEFORE INSERT ON pilgrim_refund_payout_requests
  FOR EACH ROW EXECUTE FUNCTION refund_payout_has_balance();

-- Only workflow/provider evidence may change. Beneficiary, amount, destination
-- and encrypted account details are the immutable payment instruction.
-- +goose StatementBegin
CREATE FUNCTION refund_payout_guard() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF current_setting('app.allow_ledger_purge', true) = 'on' THEN RETURN OLD; END IF;
    RAISE EXCEPTION 'refund payout requests are financial records and cannot be deleted';
  END IF;
  IF NEW.operator_id <> OLD.operator_id
     OR NEW.beneficiary_kind <> OLD.beneficiary_kind
     OR NEW.pilgrim_id IS DISTINCT FROM OLD.pilgrim_id
     OR NEW.agent_id IS DISTINCT FROM OLD.agent_id
     OR NEW.amount_idr <> OLD.amount_idr OR NEW.method <> OLD.method
     OR NEW.note <> OLD.note OR NEW.idempotency_key <> OLD.idempotency_key
     OR NEW.requested_by_user_id <> OLD.requested_by_user_id
     OR NEW.destination_channel <> OLD.destination_channel
     OR NEW.destination_account_holder <> OLD.destination_account_holder
     OR NEW.destination_account_encrypted <> OLD.destination_account_encrypted
     OR NEW.destination_account_last4 <> OLD.destination_account_last4
     OR NEW.created_at <> OLD.created_at THEN
    RAISE EXCEPTION 'refund payout instruction is immutable';
  END IF;
  IF NEW.status <> OLD.status AND NOT (
       (OLD.status = 'REQUESTED' AND NEW.status IN ('PROCESSING', 'FAILED'))
    OR (OLD.status = 'PROCESSING' AND NEW.status IN ('PAID', 'FAILED'))
    OR (OLD.status = 'PAID' AND NEW.status = 'REVERSED')
  ) THEN
    RAISE EXCEPTION 'invalid refund payout transition from % to %', OLD.status, NEW.status
      USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.status IN ('FAILED', 'REVERSED') AND NEW IS DISTINCT FROM OLD THEN
    RAISE EXCEPTION 'resolved refund payout requests are immutable';
  END IF;
  IF OLD.status = 'PAID' AND NEW.status <> 'REVERSED' AND NEW IS DISTINCT FROM OLD THEN
    RAISE EXCEPTION 'paid refund payout can only receive a provider reversal';
  END IF;
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER refund_payout_guard_trigger
  BEFORE UPDATE OR DELETE ON pilgrim_refund_payout_requests
  FOR EACH ROW EXECUTE FUNCTION refund_payout_guard();

-- Terminal gateway states must agree with the corresponding append-only
-- balance ledger in the same transaction.
-- +goose StatementBegin
CREATE FUNCTION refund_payout_has_ledger_movements() RETURNS trigger AS $$
DECLARE
  has_withdrawal BOOLEAN;
  has_reversal BOOLEAN;
BEGIN
  IF NEW.beneficiary_kind = 'PILGRIM' THEN
    SELECT EXISTS (SELECT 1 FROM pilgrim_balance_entries e
      WHERE e.pilgrim_id = NEW.pilgrim_id AND e.kind = 'WITHDRAWAL'
        AND e.amount_idr = -NEW.amount_idr
        AND e.idempotency_key = 'refund-payout-' || NEW.id::text) INTO has_withdrawal;
    SELECT EXISTS (SELECT 1 FROM pilgrim_balance_entries e
      WHERE e.pilgrim_id = NEW.pilgrim_id AND e.kind = 'ADJUSTMENT'
        AND e.amount_idr = NEW.amount_idr
        AND e.idempotency_key = 'refund-payout-reversed-' || NEW.id::text) INTO has_reversal;
  ELSE
    SELECT EXISTS (SELECT 1 FROM agent_refund_balance_entries e
      WHERE e.agent_id = NEW.agent_id AND e.kind = 'WITHDRAWAL'
        AND e.amount_idr = -NEW.amount_idr
        AND e.idempotency_key = 'refund-payout-' || NEW.id::text) INTO has_withdrawal;
    SELECT EXISTS (SELECT 1 FROM agent_refund_balance_entries e
      WHERE e.agent_id = NEW.agent_id AND e.kind = 'ADJUSTMENT'
        AND e.amount_idr = NEW.amount_idr
        AND e.idempotency_key = 'refund-payout-reversed-' || NEW.id::text) INTO has_reversal;
  END IF;
  IF NEW.status IN ('PAID', 'REVERSED') AND NOT has_withdrawal THEN
    RAISE EXCEPTION 'paid refund payout % has no matching withdrawal', NEW.id
      USING ERRCODE = 'check_violation';
  END IF;
  IF NEW.status = 'REVERSED' AND NOT has_reversal THEN
    RAISE EXCEPTION 'reversed refund payout % has no matching balance restoration', NEW.id
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER refund_payout_ledger_trigger
  AFTER INSERT OR UPDATE ON pilgrim_refund_payout_requests
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION refund_payout_has_ledger_movements();

REVOKE DELETE, TRUNCATE ON agent_refund_balance_entries FROM safrat_app;

-- +goose Down
DROP TRIGGER IF EXISTS refund_payout_ledger_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS refund_payout_has_ledger_movements();
DROP TRIGGER IF EXISTS refund_payout_guard_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS refund_payout_guard();
DROP TRIGGER IF EXISTS refund_payout_balance_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS refund_payout_has_balance();
DROP INDEX IF EXISTS refund_payout_provider_id_idx;
DROP INDEX IF EXISTS refund_payout_agent_idx;
DROP INDEX IF EXISTS refund_payout_agent_idempotency_idx;
DROP INDEX IF EXISTS refund_payout_pilgrim_idempotency_idx;
ALTER TABLE pilgrim_refund_payout_requests DROP CONSTRAINT IF EXISTS refund_payout_exactly_one_beneficiary_check;
ALTER TABLE pilgrim_refund_payout_requests DROP CONSTRAINT IF EXISTS refund_payout_beneficiary_kind_check;
ALTER TABLE pilgrim_refund_payout_requests DROP CONSTRAINT pilgrim_refund_payout_requests_status_check;
ALTER TABLE pilgrim_refund_payout_requests ADD CONSTRAINT pilgrim_refund_payout_requests_status_check
  CHECK (status IN ('REQUESTED', 'PROCESSING', 'PAID', 'FAILED'));
ALTER TABLE pilgrim_refund_payout_requests
  DROP COLUMN proof_url,
  DROP COLUMN provider_last_attempt_at,
  DROP COLUMN provider_failure_code,
  DROP COLUMN provider_status,
  DROP COLUMN provider_payout_id,
  DROP COLUMN provider,
  DROP COLUMN destination_account_last4,
  DROP COLUMN destination_account_encrypted,
  DROP COLUMN destination_account_holder,
  DROP COLUMN destination_channel,
  DROP COLUMN agent_id,
  DROP COLUMN beneficiary_kind,
  ALTER COLUMN pilgrim_id SET NOT NULL;
ALTER TABLE pilgrim_refund_payout_requests
  ADD CONSTRAINT pilgrim_refund_payout_requests_pilgrim_id_idempotency_key_key UNIQUE (pilgrim_id, idempotency_key);
DROP TRIGGER IF EXISTS agent_refund_balance_append_only ON agent_refund_balance_entries;
DROP TABLE IF EXISTS agent_refund_balance_entries;

-- Restore migration 118's invariants so a rollback that remains on version
-- 118 is still safe, rather than leaving the old table writable without its
-- balance and append-only guards.
-- +goose StatementBegin
CREATE FUNCTION pilgrim_refund_payout_has_balance() RETURNS trigger AS $$
DECLARE actual_operator UUID; current_balance BIGINT; already_reserved BIGINT;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.pilgrim_id::text, 0));
  SELECT operator_id INTO actual_operator FROM pilgrims WHERE id = NEW.pilgrim_id;
  IF actual_operator IS NULL OR actual_operator <> NEW.operator_id THEN
    RAISE EXCEPTION 'payout request operator does not own pilgrim %', NEW.pilgrim_id USING ERRCODE = 'check_violation';
  END IF;
  SELECT COALESCE(SUM(amount_idr),0)::bigint INTO current_balance FROM pilgrim_balance_entries WHERE pilgrim_id=NEW.pilgrim_id;
  SELECT COALESCE(SUM(amount_idr),0)::bigint INTO already_reserved FROM pilgrim_refund_payout_requests WHERE pilgrim_id=NEW.pilgrim_id AND status IN ('REQUESTED','PROCESSING');
  IF NEW.amount_idr > current_balance-already_reserved THEN
    RAISE EXCEPTION 'refund payout exceeds available balance' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER pilgrim_refund_payout_balance_trigger BEFORE INSERT ON pilgrim_refund_payout_requests FOR EACH ROW EXECUTE FUNCTION pilgrim_refund_payout_has_balance();

-- +goose StatementBegin
CREATE FUNCTION pilgrim_refund_payout_guard() RETURNS trigger AS $$
BEGIN
  IF TG_OP='DELETE' THEN
    IF current_setting('app.allow_ledger_purge',true)='on' THEN RETURN OLD; END IF;
    RAISE EXCEPTION 'refund payout requests are financial records and cannot be deleted';
  END IF;
  IF NEW.operator_id<>OLD.operator_id OR NEW.pilgrim_id<>OLD.pilgrim_id OR NEW.amount_idr<>OLD.amount_idr OR NEW.method<>OLD.method OR NEW.note<>OLD.note OR NEW.idempotency_key<>OLD.idempotency_key OR NEW.requested_by_user_id<>OLD.requested_by_user_id OR NEW.created_at<>OLD.created_at THEN
    RAISE EXCEPTION 'refund payout request identity and amount are immutable';
  END IF;
  IF NEW.status<>OLD.status AND NOT ((OLD.status='REQUESTED' AND NEW.status IN ('PROCESSING','FAILED')) OR (OLD.status='PROCESSING' AND NEW.status IN ('PAID','FAILED'))) THEN
    RAISE EXCEPTION 'invalid refund payout transition from % to %',OLD.status,NEW.status USING ERRCODE='check_violation';
  END IF;
  IF OLD.status IN ('PAID','FAILED') AND NEW IS DISTINCT FROM OLD THEN RAISE EXCEPTION 'resolved refund payout requests are immutable'; END IF;
  NEW.updated_at=NOW(); RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER pilgrim_refund_payout_guard_trigger BEFORE UPDATE OR DELETE ON pilgrim_refund_payout_requests FOR EACH ROW EXECUTE FUNCTION pilgrim_refund_payout_guard();

-- +goose StatementBegin
CREATE FUNCTION paid_refund_payout_has_withdrawal() RETURNS trigger AS $$
BEGIN
  IF NEW.status='PAID' AND NOT EXISTS (SELECT 1 FROM pilgrim_balance_entries e WHERE e.pilgrim_id=NEW.pilgrim_id AND e.kind='WITHDRAWAL' AND e.amount_idr=-NEW.amount_idr AND e.idempotency_key='pilgrim-payout-'||NEW.id::text) THEN
    RAISE EXCEPTION 'paid refund payout % has no matching withdrawal ledger entry',NEW.id USING ERRCODE='check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER paid_refund_payout_has_withdrawal_trigger AFTER INSERT OR UPDATE ON pilgrim_refund_payout_requests DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION paid_refund_payout_has_withdrawal();
