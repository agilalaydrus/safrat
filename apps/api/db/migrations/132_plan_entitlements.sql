-- +goose Up
-- Limits are data, not conditionals scattered across handlers. NULL means
-- unlimited; zero is a real limit (not an accidental synonym for unlimited).
CREATE TABLE plan_limits (
  plan          plan PRIMARY KEY,
  max_pilgrims  INTEGER CHECK (max_pilgrims IS NULL OR max_pilgrims >= 0),
  max_branches  INTEGER CHECK (max_branches IS NULL OR max_branches >= 0),
  feature_flags JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO plan_limits (plan, max_pilgrims, max_branches, feature_flags) VALUES
  ('STARTER', 200, 0, '{"branches": false}'::jsonb),
  ('GROWTH',  500, 3, '{"branches": true}'::jsonb),
  ('PRO',    NULL, NULL, '{"branches": true}'::jsonb);

-- Overrides are intentionally sparse. The merged JSON makes false overrides
-- meaningful, unlike a bool column where false and absent are easy to blur.
CREATE TABLE plan_overrides (
  operator_id            UUID PRIMARY KEY REFERENCES operators(id) ON DELETE CASCADE,
  max_pilgrims           INTEGER CHECK (max_pilgrims IS NULL OR max_pilgrims >= 0),
  max_branches           INTEGER CHECK (max_branches IS NULL OR max_branches >= 0),
  feature_flag_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
  note                   TEXT NOT NULL DEFAULT '',
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The service provides a clear error before an action is attempted; this
-- trigger is the concurrent-safe final authority. The advisory transaction
-- lock serialises quota-consuming writes for one operator, so two simultaneous
-- creates cannot both pass a stale COUNT(*) check.
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

CREATE TRIGGER pilgrims_entitlement_guard
  BEFORE INSERT ON pilgrims
  FOR EACH ROW EXECUTE FUNCTION assert_operator_entitlement();
CREATE TRIGGER branches_entitlement_guard
  BEFORE INSERT OR UPDATE OF is_active, operator_id ON branches
  FOR EACH ROW WHEN (NEW.is_active)
  EXECUTE FUNCTION assert_operator_entitlement();

-- +goose Down
DROP TRIGGER IF EXISTS branches_entitlement_guard ON branches;
DROP TRIGGER IF EXISTS pilgrims_entitlement_guard ON pilgrims;
DROP FUNCTION IF EXISTS assert_operator_entitlement();
DROP TABLE IF EXISTS plan_overrides;
DROP TABLE IF EXISTS plan_limits;
