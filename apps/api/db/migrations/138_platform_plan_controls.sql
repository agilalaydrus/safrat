-- +goose Up

-- An exception must say who granted it, why, and when it stops applying.
-- Existing rows predate that contract, so preserve them with an explicit
-- legacy reason instead of making the migration fail or inventing an actor.
UPDATE plan_overrides
SET note = 'legacy override tanpa alasan'
WHERE length(trim(note)) = 0;

ALTER TABLE plan_overrides
  ALTER COLUMN note DROP DEFAULT,
  ADD COLUMN expires_at TIMESTAMPTZ,
  ADD COLUMN updated_by TEXT NOT NULL DEFAULT '',
  ADD CONSTRAINT plan_overrides_note_required CHECK (length(trim(note)) > 0);

CREATE INDEX plan_overrides_expiry_idx
  ON plan_overrides (expires_at, operator_id)
  WHERE expires_at IS NOT NULL;

-- Evidence for high-impact platform decisions. SET_PLAN_LIMIT uses this from
-- A1; the remaining kinds are declared now so later work extends the workflow
-- rather than replacing its evidence table.
CREATE TABLE privileged_actions (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kind             TEXT NOT NULL CHECK (kind IN ('SUSPEND','DELETE_TENANT','SET_PLAN_LIMIT','SET_SETTLEMENT')),
  payload          JSONB NOT NULL,
  reason           TEXT NOT NULL CHECK (length(trim(reason)) > 0),
  requested_by     TEXT NOT NULL CHECK (length(trim(requested_by)) > 0),
  requested_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  approved_by      TEXT NOT NULL CHECK (length(trim(approved_by)) > 0),
  approved_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  executed_at      TIMESTAMPTZ,
  idempotency_key  TEXT NOT NULL CHECK (length(trim(idempotency_key)) BETWEEN 8 AND 128),
  UNIQUE (requested_by, kind, idempotency_key)
);

-- Every retryable platform mutation writes its key into the audit metadata in
-- the same transaction. This unique expression is the database authority;
-- clients and SELECT-before-write checks are not.
CREATE UNIQUE INDEX audit_logs_platform_idempotency_idx
  ON audit_logs (user_id, action, (metadata->>'idempotency_key'))
  WHERE metadata ? 'idempotency_key';

-- Expired overrides must stop at the enforcement layer, not merely disappear
-- from the UI. Rebuild the quota trigger with the active-row predicate.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_operator_entitlement() RETURNS trigger AS $$
DECLARE
  pilgrim_limit INTEGER;
  branch_limit INTEGER;
  branch_feature BOOLEAN;
  used_count INTEGER;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.operator_id::text, 0));
  SELECT COALESCE(o.max_pilgrims, l.max_pilgrims),
         COALESCE(o.max_branches, l.max_branches),
         COALESCE(((l.feature_flags || COALESCE(o.feature_flag_overrides, '{}'::jsonb))->>'branches')::boolean, false)
  INTO pilgrim_limit, branch_limit, branch_feature
  FROM operators op
  JOIN plan_limits l ON l.plan = op.plan
  LEFT JOIN plan_overrides o ON o.operator_id = op.id
    AND (o.expires_at IS NULL OR o.expires_at > NOW())
  WHERE op.id = NEW.operator_id;

  IF TG_TABLE_NAME = 'pilgrims' THEN
    IF pilgrim_limit IS NOT NULL THEN
      SELECT COUNT(*) INTO used_count FROM pilgrims WHERE operator_id = NEW.operator_id;
      IF used_count >= pilgrim_limit THEN
        RAISE EXCEPTION 'pilgrim limit reached for operator %', NEW.operator_id
          USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_pilgrim_limit';
      END IF;
    END IF;
  ELSIF TG_TABLE_NAME = 'branches' THEN
    IF NOT branch_feature THEN
      RAISE EXCEPTION 'branches are not enabled for operator %', NEW.operator_id
        USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_branch_feature';
    END IF;
    IF branch_limit IS NOT NULL THEN
      SELECT COUNT(*) INTO used_count FROM branches
      WHERE operator_id = NEW.operator_id AND is_active AND id <> NEW.id;
      IF used_count >= branch_limit THEN
        RAISE EXCEPTION 'branch limit reached for operator %', NEW.operator_id
          USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_branch_limit';
      END IF;
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_installments_entitled() RETURNS trigger AS $$
DECLARE enabled BOOLEAN;
BEGIN
  SELECT COALESCE(
    ((l.feature_flags || COALESCE(o.feature_flag_overrides, '{}'::jsonb))->>'installments')::boolean,
    false
  ) INTO enabled
  FROM operators op
  JOIN plan_limits l ON l.plan = op.plan
  LEFT JOIN plan_overrides o ON o.operator_id = op.id
    AND (o.expires_at IS NULL OR o.expires_at > NOW())
  WHERE op.id = NEW.operator_id;

  IF enabled IS DISTINCT FROM TRUE THEN
    RAISE EXCEPTION 'installments are not enabled for operator %', NEW.operator_id
      USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_installments_feature';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_crm_entitled() RETURNS trigger AS $$
DECLARE enabled BOOLEAN;
BEGIN
  SELECT COALESCE(
    ((l.feature_flags || COALESCE(o.feature_flag_overrides, '{}'::jsonb))->>'crm')::boolean,
    false
  ) INTO enabled
  FROM operators op
  JOIN plan_limits l ON l.plan = op.plan
  LEFT JOIN plan_overrides o ON o.operator_id = op.id
    AND (o.expires_at IS NULL OR o.expires_at > NOW())
  WHERE op.id = NEW.operator_id;

  IF enabled IS DISTINCT FROM TRUE THEN
    RAISE EXCEPTION 'crm is not enabled for operator %', NEW.operator_id
      USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_crm_feature';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Evidence tables are append-only to the application credential. The owner
-- retains maintenance authority for migrations and retention procedures.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE UPDATE, DELETE, TRUNCATE ON privileged_actions FROM safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    GRANT UPDATE, DELETE ON privileged_actions TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

DROP INDEX IF EXISTS audit_logs_platform_idempotency_idx;
DROP TABLE IF EXISTS privileged_actions;

-- Restore the pre-138 functions before removing the column they reference.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_operator_entitlement() RETURNS trigger AS $$
DECLARE
  pilgrim_limit INTEGER;
  branch_limit INTEGER;
  branch_feature BOOLEAN;
  used_count INTEGER;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.operator_id::text, 0));
  SELECT COALESCE(o.max_pilgrims, l.max_pilgrims),
         COALESCE(o.max_branches, l.max_branches),
         COALESCE(((l.feature_flags || COALESCE(o.feature_flag_overrides, '{}'::jsonb))->>'branches')::boolean, false)
  INTO pilgrim_limit, branch_limit, branch_feature
  FROM operators op
  JOIN plan_limits l ON l.plan = op.plan
  LEFT JOIN plan_overrides o ON o.operator_id = op.id
  WHERE op.id = NEW.operator_id;

  IF TG_TABLE_NAME = 'pilgrims' THEN
    IF pilgrim_limit IS NOT NULL THEN
      SELECT COUNT(*) INTO used_count FROM pilgrims WHERE operator_id = NEW.operator_id;
      IF used_count >= pilgrim_limit THEN
        RAISE EXCEPTION 'pilgrim limit reached for operator %', NEW.operator_id
          USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_pilgrim_limit';
      END IF;
    END IF;
  ELSIF TG_TABLE_NAME = 'branches' THEN
    IF NOT branch_feature THEN
      RAISE EXCEPTION 'branches are not enabled for operator %', NEW.operator_id
        USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_branch_feature';
    END IF;
    IF branch_limit IS NOT NULL THEN
      SELECT COUNT(*) INTO used_count FROM branches
      WHERE operator_id = NEW.operator_id AND is_active AND id <> NEW.id;
      IF used_count >= branch_limit THEN
        RAISE EXCEPTION 'branch limit reached for operator %', NEW.operator_id
          USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_branch_limit';
      END IF;
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_installments_entitled() RETURNS trigger AS $$
DECLARE enabled BOOLEAN;
BEGIN
  SELECT COALESCE(((l.feature_flags || COALESCE(o.feature_flag_overrides, '{}'::jsonb))->>'installments')::boolean, false)
  INTO enabled
  FROM operators op JOIN plan_limits l ON l.plan = op.plan
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

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_crm_entitled() RETURNS trigger AS $$
DECLARE enabled BOOLEAN;
BEGIN
  SELECT COALESCE(((l.feature_flags || COALESCE(o.feature_flag_overrides, '{}'::jsonb))->>'crm')::boolean, false)
  INTO enabled
  FROM operators op JOIN plan_limits l ON l.plan = op.plan
  LEFT JOIN plan_overrides o ON o.operator_id = op.id
  WHERE op.id = NEW.operator_id;
  IF enabled IS DISTINCT FROM TRUE THEN
    RAISE EXCEPTION 'crm is not enabled for operator %', NEW.operator_id
      USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_crm_feature';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP INDEX IF EXISTS plan_overrides_expiry_idx;
ALTER TABLE plan_overrides
  DROP CONSTRAINT IF EXISTS plan_overrides_note_required,
  DROP COLUMN IF EXISTS updated_by,
  DROP COLUMN IF EXISTS expires_at,
  ALTER COLUMN note SET DEFAULT '';
