-- +goose Up

-- Refund payout destinations use the same AES key as KYC identities. Stamp
-- every newly-written ciphertext with the key fingerprint so a deployment can
-- identify a wrong key before a payout is dispatched, and so rotation can be
-- resumed without guessing which rows have already moved.
ALTER TABLE pilgrim_refund_payout_requests
  ADD COLUMN destination_key_fingerprint TEXT NOT NULL DEFAULT '';

CREATE INDEX refund_payout_destination_key_idx
  ON pilgrim_refund_payout_requests (destination_key_fingerprint)
  WHERE destination_account_encrypted <> '';

-- Existing encrypted rows predate the fingerprint column and therefore keep
-- an empty stamp until rotatekyc opens them with the old key and re-seals them.
-- NOT VALID preserves those rows while still enforcing the invariant for new
-- inserts and every row touched from this point onward.
ALTER TABLE pilgrim_refund_payout_requests
  ADD CONSTRAINT refund_payout_destination_key_check CHECK (
    (destination_account_encrypted = '' AND destination_key_fingerprint = '')
    OR
    (destination_account_encrypted LIKE 'v1.%'
      AND destination_key_fingerprint ~ '^[0-9a-f]{8}$')
  ) NOT VALID;

-- Destination instructions are financial evidence and remain immutable. Key
-- rotation is the sole exception: an administrative connection may replace
-- only the ciphertext and its fingerprint, with all other columns byte-for-
-- byte unchanged. The production application role cannot enable this path.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION refund_payout_guard() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF current_setting('app.allow_ledger_purge', true) = 'on' THEN RETURN OLD; END IF;
    RAISE EXCEPTION 'refund payout requests are financial records and cannot be deleted';
  END IF;

  IF current_setting('app.allow_payout_key_rotation', true) = 'on'
     AND current_user <> 'safrat_app'
     AND NEW.destination_account_encrypted IS DISTINCT FROM OLD.destination_account_encrypted
     AND NEW.destination_key_fingerprint IS DISTINCT FROM OLD.destination_key_fingerprint
     AND NEW.destination_account_encrypted LIKE 'v1.%'
     AND NEW.destination_key_fingerprint ~ '^[0-9a-f]{8}$'
     AND (to_jsonb(NEW) - ARRAY['destination_account_encrypted', 'destination_key_fingerprint', 'updated_at'])
       = (to_jsonb(OLD) - ARRAY['destination_account_encrypted', 'destination_key_fingerprint', 'updated_at']) THEN
    NEW.updated_at = NOW();
    RETURN NEW;
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
     OR NEW.destination_key_fingerprint <> OLD.destination_key_fingerprint
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

-- +goose Down

-- Restore migration 119's guard before removing the column it references.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION refund_payout_guard() RETURNS trigger AS $$
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

ALTER TABLE pilgrim_refund_payout_requests
  DROP CONSTRAINT refund_payout_destination_key_check;
DROP INDEX refund_payout_destination_key_idx;
ALTER TABLE pilgrim_refund_payout_requests
  DROP COLUMN destination_key_fingerprint;
