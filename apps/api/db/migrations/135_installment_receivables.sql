-- +goose Up

-- Installments are available on every commercial plan (they are a market
-- baseline, not an upsell), but still live in entitlement data so an explicit
-- operator override can disable them without a code deploy.
UPDATE plan_limits
SET feature_flags = jsonb_set(feature_flags, '{installments}', 'true'::jsonb, true),
    updated_at = NOW();

CREATE TABLE installment_plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id UUID NOT NULL REFERENCES seasons(id) ON DELETE RESTRICT,
  pilgrim_id UUID NOT NULL REFERENCES pilgrims(id) ON DELETE RESTRICT,
  branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT,
  scheme TEXT NOT NULL CHECK (scheme IN ('FULL','DP_50','INSTALLMENT_6X','INSTALLMENT_12X','CASH_BONUS')),
  gross_amount_idr BIGINT NOT NULL CHECK (gross_amount_idr > 0),
  cash_bonus_idr BIGINT NOT NULL DEFAULT 0 CHECK (cash_bonus_idr >= 0),
  payable_amount_idr BIGINT GENERATED ALWAYS AS (gross_amount_idr - cash_bonus_idr) STORED,
  first_due_date DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','CANCELLED')),
  cancelled_at TIMESTAMPTZ,
  cancelled_by_user_id TEXT,
  cancellation_reason TEXT NOT NULL DEFAULT '',
  created_by_user_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL CHECK (length(trim(idempotency_key)) > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (payable_amount_idr > 0),
  CHECK (
    (scheme = 'CASH_BONUS' AND cash_bonus_idr > 0)
    OR (scheme <> 'CASH_BONUS' AND cash_bonus_idr = 0)
  ),
  CHECK (
    (status = 'ACTIVE' AND cancelled_at IS NULL AND cancelled_by_user_id IS NULL AND cancellation_reason = '')
    OR
    (status = 'CANCELLED' AND cancelled_at IS NOT NULL AND cancelled_by_user_id IS NOT NULL AND cancellation_reason <> '')
  )
);

CREATE UNIQUE INDEX installment_plans_active_pilgrim_idx
  ON installment_plans (pilgrim_id) WHERE status = 'ACTIVE';
CREATE UNIQUE INDEX installment_plans_idempotency_idx
  ON installment_plans (operator_id, idempotency_key);
CREATE INDEX installment_plans_receivable_idx
  ON installment_plans (operator_id, season_id, branch_id, created_at DESC)
  WHERE status = 'ACTIVE';

CREATE TABLE installments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  plan_id UUID NOT NULL REFERENCES installment_plans(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT,
  installment_number INTEGER NOT NULL CHECK (installment_number > 0),
  label TEXT NOT NULL CHECK (length(trim(label)) > 0),
  due_date DATE NOT NULL,
  amount_due_idr BIGINT NOT NULL CHECK (amount_due_idr > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (plan_id, installment_number)
);
CREATE INDEX installments_due_idx
  ON installments (operator_id, due_date, branch_id);

CREATE SEQUENCE installment_receipt_seq;

CREATE TABLE installment_payment_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  plan_id UUID NOT NULL REFERENCES installment_plans(id) ON DELETE RESTRICT,
  installment_id UUID NOT NULL REFERENCES installments(id) ON DELETE RESTRICT,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT,
  kind TEXT NOT NULL CHECK (kind IN ('PAYMENT','REVERSAL')),
  amount_idr BIGINT NOT NULL CHECK (amount_idr <> 0),
  method TEXT NOT NULL CHECK (method IN ('CASH','BANK_TRANSFER','XENDIT')),
  reference TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  original_payment_id UUID REFERENCES installment_payment_entries(id) ON DELETE RESTRICT,
  verified_by_user_id TEXT NOT NULL CHECK (length(trim(verified_by_user_id)) > 0),
  idempotency_key TEXT NOT NULL CHECK (length(trim(idempotency_key)) > 0),
  receipt_number TEXT NOT NULL DEFAULT (
    'PAY-' || to_char(NOW(), 'YYYYMM') || '-' || lpad(nextval('installment_receipt_seq')::text, 7, '0')
  ),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (
    (kind = 'PAYMENT' AND amount_idr > 0 AND original_payment_id IS NULL)
    OR
    (kind = 'REVERSAL' AND amount_idr < 0 AND original_payment_id IS NOT NULL)
  ),
  UNIQUE (receipt_number)
);
CREATE UNIQUE INDEX installment_payment_idempotency_idx
  ON installment_payment_entries (operator_id, idempotency_key);
CREATE UNIQUE INDEX installment_payment_one_reversal_idx
  ON installment_payment_entries (original_payment_id)
  WHERE kind = 'REVERSAL';
CREATE INDEX installment_payment_plan_idx
  ON installment_payment_entries (plan_id, created_at DESC);
CREATE INDEX installment_payment_installment_idx
  ON installment_payment_entries (installment_id, created_at ASC);

CREATE TRIGGER installment_plans_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON installment_plans
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();
CREATE TRIGGER installments_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON installments
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();
CREATE TRIGGER installment_payments_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON installment_payment_entries
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_installments_entitled() RETURNS trigger AS $$
DECLARE
  enabled BOOLEAN;
BEGIN
  SELECT COALESCE(
    ((l.feature_flags || COALESCE(o.feature_flag_overrides, '{}'::jsonb))->>'installments')::boolean,
    false
  ) INTO enabled
  FROM operators op
  JOIN plan_limits l ON l.plan = op.plan
  LEFT JOIN plan_overrides o ON o.operator_id = op.id
  WHERE op.id = NEW.operator_id;

  IF enabled IS DISTINCT FROM TRUE THEN
    RAISE EXCEPTION 'installments are not enabled for operator %', NEW.operator_id
      USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_installments_feature';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER installment_plans_entitlement_guard
  BEFORE INSERT ON installment_plans
  FOR EACH ROW EXECUTE FUNCTION assert_installments_entitled();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION freeze_installment_plan_identity() RETURNS trigger AS $$
BEGIN
  IF NEW.operator_id <> OLD.operator_id
     OR NEW.season_id <> OLD.season_id
     OR NEW.pilgrim_id <> OLD.pilgrim_id
     OR NEW.branch_id IS DISTINCT FROM OLD.branch_id
     OR NEW.scheme <> OLD.scheme
     OR NEW.gross_amount_idr <> OLD.gross_amount_idr
     OR NEW.cash_bonus_idr <> OLD.cash_bonus_idr
     OR NEW.first_due_date <> OLD.first_due_date
     OR NEW.created_by_user_id <> OLD.created_by_user_id
     OR NEW.idempotency_key <> OLD.idempotency_key
     OR NEW.created_at <> OLD.created_at THEN
    RAISE EXCEPTION 'installment plan identity and financial terms are immutable';
  END IF;
  IF OLD.status = 'CANCELLED' THEN
    RAISE EXCEPTION 'cancelled installment plan is immutable';
  END IF;
  IF NEW.status <> OLD.status AND NEW.status <> 'CANCELLED' THEN
    RAISE EXCEPTION 'installment plan may only transition from ACTIVE to CANCELLED';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER installment_plans_freeze_identity
  BEFORE UPDATE ON installment_plans
  FOR EACH ROW EXECUTE FUNCTION freeze_installment_plan_identity();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION installment_fact_is_append_only() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' AND current_setting('app.allow_ledger_purge', true) = 'on' THEN
    RETURN OLD;
  END IF;
  RAISE EXCEPTION 'installment facts are append-only; append a reversal or replacement plan instead of % on %', TG_OP, TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER installments_append_only
  BEFORE UPDATE OR DELETE ON installments
  FOR EACH ROW EXECUTE FUNCTION installment_fact_is_append_only();
CREATE TRIGGER installment_payments_append_only
  BEFORE UPDATE OR DELETE ON installment_payment_entries
  FOR EACH ROW EXECUTE FUNCTION installment_fact_is_append_only();

-- The schedule must add up exactly to the contractual payable amount. Both
-- triggers are deferred so a transaction can insert the plan and all schedule
-- rows before the invariant is evaluated at COMMIT.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_installment_schedule_total() RETURNS trigger AS $$
DECLARE
  target_plan UUID;
  expected BIGINT;
  actual BIGINT;
  row_count INTEGER;
BEGIN
  IF TG_TABLE_NAME = 'installment_plans' THEN
    target_plan := NEW.id;
  ELSE
    target_plan := COALESCE(NEW.plan_id, OLD.plan_id);
  END IF;

  SELECT payable_amount_idr INTO expected FROM installment_plans WHERE id = target_plan;
  IF expected IS NULL THEN
    RETURN NULL;
  END IF;
  SELECT COALESCE(SUM(amount_due_idr), 0), COUNT(*)
  INTO actual, row_count
  FROM installments WHERE plan_id = target_plan;

  IF row_count = 0 OR actual <> expected THEN
    RAISE EXCEPTION 'installment schedule for plan % totals %, expected %', target_plan, actual, expected
      USING ERRCODE = 'check_violation', CONSTRAINT = 'installment_schedule_total';
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER installment_plan_needs_balanced_schedule
  AFTER INSERT ON installment_plans
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION assert_installment_schedule_total();
CREATE CONSTRAINT TRIGGER installment_schedule_stays_balanced
  AFTER INSERT OR UPDATE OR DELETE ON installments
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION assert_installment_schedule_total();

-- Serialize concurrent payments on the installment row, then validate the
-- resulting balance. Idempotent replays return early so the already-counted
-- first insert is not mistaken for an overpayment before ON CONFLICT runs.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION guard_installment_payment_entry() RETURNS trigger AS $$
DECLARE
  installment_due BIGINT;
  installment_plan UUID;
  installment_operator UUID;
  installment_branch UUID;
  plan_status TEXT;
  already_paid BIGINT;
  original installment_payment_entries%ROWTYPE;
BEGIN
  IF EXISTS (
    SELECT 1 FROM installment_payment_entries e
    WHERE e.operator_id = NEW.operator_id AND e.idempotency_key = NEW.idempotency_key
  ) THEN
    RETURN NEW;
  END IF;

  SELECT i.amount_due_idr, i.plan_id, i.operator_id, i.branch_id, p.status
  INTO installment_due, installment_plan, installment_operator, installment_branch, plan_status
  FROM installments i
  JOIN installment_plans p ON p.id = i.plan_id
  WHERE i.id = NEW.installment_id
  FOR UPDATE OF i;

  IF installment_due IS NULL THEN
    RAISE EXCEPTION 'installment % does not exist', NEW.installment_id
      USING ERRCODE = 'foreign_key_violation';
  END IF;
  IF NEW.plan_id <> installment_plan OR NEW.operator_id <> installment_operator
     OR NEW.branch_id IS DISTINCT FROM installment_branch THEN
    RAISE EXCEPTION 'payment entry ownership does not match installment';
  END IF;
  IF plan_status <> 'ACTIVE' THEN
    RAISE EXCEPTION 'installment plan is not active' USING ERRCODE = 'check_violation';
  END IF;

  IF NEW.kind = 'REVERSAL' THEN
    IF EXISTS (
      SELECT 1 FROM installment_payment_entries e
      WHERE e.kind = 'REVERSAL' AND e.original_payment_id = NEW.original_payment_id
    ) THEN
      RAISE EXCEPTION 'payment % has already been reversed', NEW.original_payment_id
        USING ERRCODE = 'unique_violation', CONSTRAINT = 'installment_payment_one_reversal_idx';
    END IF;
    SELECT * INTO original
    FROM installment_payment_entries
    WHERE id = NEW.original_payment_id
    FOR SHARE;
    IF original.id IS NULL OR original.kind <> 'PAYMENT'
       OR original.installment_id <> NEW.installment_id
       OR original.plan_id <> NEW.plan_id
       OR original.operator_id <> NEW.operator_id
       OR original.method <> NEW.method
       OR NEW.amount_idr <> -original.amount_idr THEN
      RAISE EXCEPTION 'reversal must exactly negate its original payment'
        USING ERRCODE = 'check_violation', CONSTRAINT = 'installment_reversal_matches_original';
    END IF;
  END IF;

  SELECT COALESCE(SUM(e.amount_idr), 0)
  INTO already_paid
  FROM installment_payment_entries e
  WHERE e.installment_id = NEW.installment_id;

  IF already_paid + NEW.amount_idr < 0 OR already_paid + NEW.amount_idr > installment_due THEN
    RAISE EXCEPTION 'installment payment would move net paid outside 0..amount due'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'installment_payment_balance';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER installment_payment_guard
  BEFORE INSERT ON installment_payment_entries
  FOR EACH ROW EXECUTE FUNCTION guard_installment_payment_entry();

-- pilgrims.payment_status predates the ledger and is still consumed by older
-- screens. Keep it only as a derived workflow cache, updated atomically from
-- append-only facts; it is never used to calculate a rupiah balance.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_pilgrim_installment_status() RETURNS trigger AS $$
DECLARE
  target_plan UUID;
  target_pilgrim UUID;
  payable BIGINT;
  paid BIGINT;
BEGIN
  IF TG_TABLE_NAME = 'installment_plans' THEN
    target_plan := NEW.id;
  ELSE
    target_plan := NEW.plan_id;
  END IF;
  SELECT pilgrim_id, payable_amount_idr
  INTO target_pilgrim, payable
  FROM installment_plans WHERE id = target_plan;
  SELECT COALESCE(SUM(amount_idr), 0)
  INTO paid
  FROM installment_payment_entries WHERE plan_id = target_plan;

  UPDATE pilgrims
  SET payment_status = CASE WHEN paid >= payable THEN 'PAID' WHEN paid > 0 THEN 'DP' ELSE 'UNPAID' END,
      updated_at = NOW()
  WHERE id = target_pilgrim;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER installment_plan_sync_pilgrim_status
  AFTER INSERT ON installment_plans
  FOR EACH ROW EXECUTE FUNCTION sync_pilgrim_installment_status();
CREATE TRIGGER installment_payment_sync_pilgrim_status
  AFTER INSERT ON installment_payment_entries
  FOR EACH ROW EXECUTE FUNCTION sync_pilgrim_installment_status();

-- Once an active installment contract exists, the legacy status column is a
-- projection of the ledger. Older endpoints may still submit a status, but
-- PostgreSQL refuses any value that disagrees with the financial facts.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION guard_derived_pilgrim_payment_status() RETURNS trigger AS $$
DECLARE
  active_plan UUID;
  payable BIGINT;
  paid BIGINT;
  expected TEXT;
BEGIN
  SELECT ip.id, ip.payable_amount_idr
  INTO active_plan, payable
  FROM installment_plans ip
  WHERE ip.pilgrim_id = NEW.id AND ip.status = 'ACTIVE';

  IF active_plan IS NULL THEN
    RETURN NEW;
  END IF;

  SELECT COALESCE(SUM(amount_idr), 0)
  INTO paid
  FROM installment_payment_entries
  WHERE plan_id = active_plan;

  expected := CASE WHEN paid >= payable THEN 'PAID' WHEN paid > 0 THEN 'DP' ELSE 'UNPAID' END;
  IF NEW.payment_status IS DISTINCT FROM expected THEN
    RAISE EXCEPTION 'payment status is derived from installment ledger; expected %', expected
      USING ERRCODE = 'check_violation', CONSTRAINT = 'pilgrim_payment_status_is_derived';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER pilgrim_installment_status_is_derived
  BEFORE UPDATE OF payment_status ON pilgrims
  FOR EACH ROW EXECUTE FUNCTION guard_derived_pilgrim_payment_status();

-- +goose Down
DROP TRIGGER IF EXISTS pilgrim_installment_status_is_derived ON pilgrims;
DROP FUNCTION IF EXISTS guard_derived_pilgrim_payment_status();
DROP TRIGGER IF EXISTS installment_payment_sync_pilgrim_status ON installment_payment_entries;
DROP TRIGGER IF EXISTS installment_plan_sync_pilgrim_status ON installment_plans;
DROP FUNCTION IF EXISTS sync_pilgrim_installment_status();
DROP TRIGGER IF EXISTS installment_payment_guard ON installment_payment_entries;
DROP FUNCTION IF EXISTS guard_installment_payment_entry();
DROP TRIGGER IF EXISTS installment_schedule_stays_balanced ON installments;
DROP TRIGGER IF EXISTS installment_plan_needs_balanced_schedule ON installment_plans;
DROP FUNCTION IF EXISTS assert_installment_schedule_total();
DROP TRIGGER IF EXISTS installment_payments_append_only ON installment_payment_entries;
DROP TRIGGER IF EXISTS installments_append_only ON installments;
DROP FUNCTION IF EXISTS installment_fact_is_append_only();
DROP TRIGGER IF EXISTS installment_plans_freeze_identity ON installment_plans;
DROP FUNCTION IF EXISTS freeze_installment_plan_identity();
DROP TRIGGER IF EXISTS installment_plans_entitlement_guard ON installment_plans;
DROP FUNCTION IF EXISTS assert_installments_entitled();
DROP TRIGGER IF EXISTS installment_payments_branch_matches_operator ON installment_payment_entries;
DROP TRIGGER IF EXISTS installments_branch_matches_operator ON installments;
DROP TRIGGER IF EXISTS installment_plans_branch_matches_operator ON installment_plans;
DROP TABLE IF EXISTS installment_payment_entries;
DROP SEQUENCE IF EXISTS installment_receipt_seq;
DROP TABLE IF EXISTS installments;
DROP TABLE IF EXISTS installment_plans;
UPDATE plan_limits
SET feature_flags = feature_flags - 'installments', updated_at = NOW();
