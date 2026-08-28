-- +goose Up

-- A refund credits the append-only pilgrim balance ledger, but until now that
-- balance had no operational path back to the pilgrim. These requests reserve
-- part of that balance while staff arrange the transfer/cash handover, then a
-- PAID transition appends the matching negative WITHDRAWAL entry atomically.
CREATE TABLE pilgrim_refund_payout_requests (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id           UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id            UUID NOT NULL REFERENCES pilgrims(id) ON DELETE RESTRICT,
  amount_idr            BIGINT NOT NULL CHECK (amount_idr > 0),
  method                VARCHAR(20) NOT NULL CHECK (method IN ('BANK_TRANSFER', 'EWALLET', 'CASH')),
  note                  TEXT NOT NULL DEFAULT '',
  status                VARCHAR(16) NOT NULL DEFAULT 'REQUESTED'
                        CHECK (status IN ('REQUESTED', 'PROCESSING', 'PAID', 'FAILED')),
  idempotency_key       TEXT NOT NULL,
  requested_by_user_id  TEXT NOT NULL,
  processed_by_user_id  TEXT,
  resolution_note       TEXT NOT NULL DEFAULT '',
  payment_reference     TEXT NOT NULL DEFAULT '',
  processing_at         TIMESTAMPTZ,
  resolved_at           TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (pilgrim_id, idempotency_key)
);

CREATE INDEX pilgrim_refund_payout_operator_status_idx
  ON pilgrim_refund_payout_requests (operator_id, status, created_at DESC);
CREATE INDEX pilgrim_refund_payout_pilgrim_idx
  ON pilgrim_refund_payout_requests (pilgrim_id, created_at DESC);

-- Serialise and enforce the balance decision in PostgreSQL as well as in the
-- service. A future script or endpoint that forgets the application lock still
-- cannot reserve more than the append-only ledger says is available.
-- +goose StatementBegin
CREATE FUNCTION pilgrim_refund_payout_has_balance() RETURNS trigger AS $$
DECLARE
  actual_operator UUID;
  current_balance BIGINT;
  already_reserved BIGINT;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.pilgrim_id::text, 0));

  SELECT operator_id INTO actual_operator FROM pilgrims WHERE id = NEW.pilgrim_id;
  IF actual_operator IS NULL OR actual_operator <> NEW.operator_id THEN
    RAISE EXCEPTION 'payout request operator does not own pilgrim %', NEW.pilgrim_id
      USING ERRCODE = 'check_violation';
  END IF;

  SELECT COALESCE(SUM(amount_idr), 0)::bigint INTO current_balance
  FROM pilgrim_balance_entries WHERE pilgrim_id = NEW.pilgrim_id;

  SELECT COALESCE(SUM(amount_idr), 0)::bigint INTO already_reserved
  FROM pilgrim_refund_payout_requests
  WHERE pilgrim_id = NEW.pilgrim_id AND status IN ('REQUESTED', 'PROCESSING');

  IF NEW.amount_idr > current_balance - already_reserved THEN
    RAISE EXCEPTION 'refund payout % exceeds available balance % for pilgrim %',
      NEW.amount_idr, current_balance - already_reserved, NEW.pilgrim_id
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER pilgrim_refund_payout_balance_trigger
  BEFORE INSERT ON pilgrim_refund_payout_requests
  FOR EACH ROW EXECUTE FUNCTION pilgrim_refund_payout_has_balance();

-- The request itself is a financial record. Its workflow fields may advance,
-- but its owner, amount, method and retry identity can never be rewritten, and
-- no row can be deleted. A correction is a FAILED request followed by a new
-- request, which leaves the disputed history visible.
-- +goose StatementBegin
CREATE FUNCTION pilgrim_refund_payout_guard() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF current_setting('app.allow_ledger_purge', true) = 'on' THEN
      RETURN OLD;
    END IF;
    RAISE EXCEPTION 'refund payout requests are financial records and cannot be deleted';
  END IF;

  IF NEW.operator_id <> OLD.operator_id
     OR NEW.pilgrim_id <> OLD.pilgrim_id
     OR NEW.amount_idr <> OLD.amount_idr
     OR NEW.method <> OLD.method
     OR NEW.note <> OLD.note
     OR NEW.idempotency_key <> OLD.idempotency_key
     OR NEW.requested_by_user_id <> OLD.requested_by_user_id
     OR NEW.created_at <> OLD.created_at THEN
    RAISE EXCEPTION 'refund payout request identity and amount are immutable';
  END IF;

  IF NEW.status <> OLD.status AND NOT (
       (OLD.status = 'REQUESTED' AND NEW.status IN ('PROCESSING', 'FAILED'))
    OR (OLD.status = 'PROCESSING' AND NEW.status IN ('PAID', 'FAILED'))
  ) THEN
    RAISE EXCEPTION 'invalid refund payout transition from % to %', OLD.status, NEW.status
      USING ERRCODE = 'check_violation';
  END IF;

  IF OLD.status IN ('PAID', 'FAILED') AND NEW IS DISTINCT FROM OLD THEN
    RAISE EXCEPTION 'resolved refund payout requests are immutable';
  END IF;

  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER pilgrim_refund_payout_guard_trigger
  BEFORE UPDATE OR DELETE ON pilgrim_refund_payout_requests
  FOR EACH ROW EXECUTE FUNCTION pilgrim_refund_payout_guard();

-- A request cannot commit as PAID unless the exact negative ledger movement
-- exists in the same transaction. Deferred so callers may write the request
-- and ledger in either order without exposing a half-applied result.
-- +goose StatementBegin
CREATE FUNCTION paid_refund_payout_has_withdrawal() RETURNS trigger AS $$
BEGIN
  IF NEW.status = 'PAID' AND NOT EXISTS (
    SELECT 1 FROM pilgrim_balance_entries e
    WHERE e.pilgrim_id = NEW.pilgrim_id
      AND e.kind = 'WITHDRAWAL'
      AND e.amount_idr = -NEW.amount_idr
      AND e.idempotency_key = 'pilgrim-payout-' || NEW.id::text
  ) THEN
    RAISE EXCEPTION 'paid refund payout % has no matching withdrawal ledger entry', NEW.id
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER paid_refund_payout_has_withdrawal_trigger
  AFTER INSERT OR UPDATE ON pilgrim_refund_payout_requests
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION paid_refund_payout_has_withdrawal();

-- Keep the request lifecycle writable, but never allow application code to
-- erase it. The default grants from migration 100 cover SELECT/INSERT/UPDATE.
REVOKE DELETE, TRUNCATE ON pilgrim_refund_payout_requests FROM safrat_app;

-- +goose Down
DROP TRIGGER IF EXISTS paid_refund_payout_has_withdrawal_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS paid_refund_payout_has_withdrawal();
DROP TRIGGER IF EXISTS pilgrim_refund_payout_guard_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS pilgrim_refund_payout_guard();
DROP TRIGGER IF EXISTS pilgrim_refund_payout_balance_trigger ON pilgrim_refund_payout_requests;
DROP FUNCTION IF EXISTS pilgrim_refund_payout_has_balance();
DROP TABLE IF EXISTS pilgrim_refund_payout_requests;
