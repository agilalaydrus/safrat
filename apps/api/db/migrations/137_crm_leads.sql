-- +goose Up

-- CRM is a commercial growth tool. Keep it visible to Starter operators in
-- the UI, but make Growth/Pro (or an explicit override) the authority at both
-- service and database layers.
UPDATE plan_limits
SET feature_flags = jsonb_set(
      feature_flags,
      '{crm}',
      CASE WHEN plan = 'STARTER' THEN 'false'::jsonb ELSE 'true'::jsonb END,
      true
    ),
    updated_at = NOW();

CREATE TABLE crm_leads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT,
  full_name TEXT NOT NULL CHECK (length(trim(full_name)) BETWEEN 2 AND 150),
  phone TEXT NOT NULL DEFAULT '' CHECK (length(phone) <= 40),
  email TEXT NOT NULL DEFAULT '' CHECK (length(email) <= 254),
  source TEXT NOT NULL CHECK (source IN ('WEBSITE','INSTAGRAM','WHATSAPP','WALK_IN','REFERRAL','ALUMNI','OTHER')),
  campaign TEXT NOT NULL DEFAULT '' CHECK (length(campaign) <= 120),
  stage TEXT NOT NULL DEFAULT 'NEW' CHECK (stage IN ('NEW','CONTACT','OFFER','HOT','CLOSING','CANCELLED')),
  season_id UUID REFERENCES seasons(id) ON DELETE SET NULL,
  product_id UUID REFERENCES products(id) ON DELETE SET NULL,
  assignee_user_id TEXT REFERENCES "user"(id) ON DELETE SET NULL,
  pax INTEGER NOT NULL DEFAULT 1 CHECK (pax BETWEEN 1 AND 1000),
  estimated_value_idr BIGINT NOT NULL DEFAULT 0 CHECK (estimated_value_idr >= 0),
  next_action TEXT NOT NULL DEFAULT '' CHECK (length(next_action) <= 300),
  next_follow_up_at TIMESTAMPTZ,
  last_contact_at TIMESTAMPTZ,
  closed_at TIMESTAMPTZ,
  last_activity_id UUID,
  created_by_user_id TEXT NOT NULL CHECK (length(trim(created_by_user_id)) > 0),
  idempotency_key TEXT NOT NULL CHECK (length(trim(idempotency_key)) > 0),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) = 64),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK ((stage = 'CLOSING') = (closed_at IS NOT NULL))
);

CREATE UNIQUE INDEX crm_leads_idempotency_idx ON crm_leads (operator_id, idempotency_key);
CREATE INDEX crm_leads_pipeline_idx ON crm_leads (operator_id, branch_id, stage, updated_at DESC);
CREATE INDEX crm_leads_follow_up_idx ON crm_leads (operator_id, branch_id, next_follow_up_at)
  WHERE stage NOT IN ('CLOSING','CANCELLED') AND next_follow_up_at IS NOT NULL;

CREATE TABLE crm_lead_activities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  lead_id UUID NOT NULL REFERENCES crm_leads(id) ON DELETE RESTRICT,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT,
  kind TEXT NOT NULL CHECK (kind IN ('CREATED','STAGE_CHANGED','CONTACT','NOTE','OFFER_SENT','PROFILE_UPDATED')),
  from_stage TEXT CHECK (from_stage IS NULL OR from_stage IN ('NEW','CONTACT','OFFER','HOT','CLOSING','CANCELLED')),
  to_stage TEXT CHECK (to_stage IS NULL OR to_stage IN ('NEW','CONTACT','OFFER','HOT','CLOSING','CANCELLED')),
  note TEXT NOT NULL DEFAULT '' CHECK (length(note) <= 2000),
  actor_user_id TEXT NOT NULL CHECK (length(trim(actor_user_id)) > 0),
  idempotency_key TEXT NOT NULL CHECK (length(trim(idempotency_key)) > 0),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) = 64),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (
    (kind = 'STAGE_CHANGED' AND from_stage IS NOT NULL AND to_stage IS NOT NULL AND from_stage <> to_stage)
    OR (kind <> 'STAGE_CHANGED' AND from_stage IS NULL AND to_stage IS NULL)
  )
);

ALTER TABLE crm_leads
  ADD CONSTRAINT crm_leads_last_activity_fkey
  FOREIGN KEY (last_activity_id) REFERENCES crm_lead_activities(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX crm_lead_activities_idempotency_idx
  ON crm_lead_activities (operator_id, idempotency_key);
CREATE INDEX crm_lead_activities_timeline_idx
  ON crm_lead_activities (lead_id, occurred_at DESC, created_at DESC);

CREATE TRIGGER crm_leads_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON crm_leads
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();
CREATE TRIGGER crm_activities_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON crm_lead_activities
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();

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
  WHERE op.id = NEW.operator_id;

  IF enabled IS DISTINCT FROM TRUE THEN
    RAISE EXCEPTION 'crm is not enabled for operator %', NEW.operator_id
      USING ERRCODE = 'check_violation', CONSTRAINT = 'operator_crm_feature';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER crm_leads_entitlement_guard
  BEFORE INSERT ON crm_leads
  FOR EACH ROW EXECUTE FUNCTION assert_crm_entitled();

-- Foreign keys alone do not enforce tenant ownership. This trigger prevents a
-- season, product, assignee, or activity from another operator being attached
-- to a lead even through direct SQL.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_crm_lead_relations() RETURNS trigger AS $$
DECLARE org_id TEXT;
BEGIN
  IF NEW.season_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM seasons WHERE id = NEW.season_id AND operator_id = NEW.operator_id
  ) THEN
    RAISE EXCEPTION 'crm season belongs to another operator'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_lead_season_operator';
  END IF;
  IF NEW.product_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM products
    WHERE id = NEW.product_id AND operator_id = NEW.operator_id
      AND (NEW.season_id IS NULL OR season_id = NEW.season_id)
  ) THEN
    RAISE EXCEPTION 'crm product belongs to another operator or season'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_lead_product_operator';
  END IF;
  IF NEW.assignee_user_id IS NOT NULL THEN
    SELECT better_auth_org_id INTO org_id FROM operators WHERE id = NEW.operator_id;
    IF NOT EXISTS (
      SELECT 1 FROM "member" WHERE "organizationId" = org_id AND "userId" = NEW.assignee_user_id
    ) THEN
      RAISE EXCEPTION 'crm assignee is not an operator member'
        USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_lead_assignee_operator';
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER crm_leads_relation_guard
  BEFORE INSERT OR UPDATE OF operator_id, season_id, product_id, assignee_user_id ON crm_leads
  FOR EACH ROW EXECUTE FUNCTION assert_crm_lead_relations();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_crm_activity_relations() RETURNS trigger AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM crm_leads l
    WHERE l.id = NEW.lead_id AND l.operator_id = NEW.operator_id
      AND l.branch_id IS NOT DISTINCT FROM NEW.branch_id
  ) THEN
    RAISE EXCEPTION 'crm activity scope does not match its lead'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_activity_lead_scope';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER crm_activities_relation_guard
  BEFORE INSERT ON crm_lead_activities
  FOR EACH ROW EXECUTE FUNCTION assert_crm_activity_relations();

-- A stage/profile mutation is valid only when it points at a fresh immutable
-- timeline event that describes the mutation. This makes direct UPDATE unable
-- to silently rewrite CRM history.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION guard_crm_lead_update() RETURNS trigger AS $$
DECLARE event crm_lead_activities%ROWTYPE;
DECLARE tracked_changed BOOLEAN;
BEGIN
  IF NEW.operator_id <> OLD.operator_id OR NEW.branch_id IS DISTINCT FROM OLD.branch_id
     OR NEW.created_by_user_id <> OLD.created_by_user_id
     OR NEW.idempotency_key <> OLD.idempotency_key
     OR NEW.request_fingerprint <> OLD.request_fingerprint
     OR NEW.created_at <> OLD.created_at THEN
    RAISE EXCEPTION 'crm lead identity is immutable'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_lead_identity_immutable';
  END IF;

  tracked_changed := ROW(
    NEW.full_name, NEW.phone, NEW.email, NEW.source, NEW.campaign, NEW.stage,
    NEW.season_id, NEW.product_id, NEW.assignee_user_id, NEW.pax,
    NEW.estimated_value_idr, NEW.next_action, NEW.next_follow_up_at,
    NEW.last_contact_at, NEW.closed_at
  ) IS DISTINCT FROM ROW(
    OLD.full_name, OLD.phone, OLD.email, OLD.source, OLD.campaign, OLD.stage,
    OLD.season_id, OLD.product_id, OLD.assignee_user_id, OLD.pax,
    OLD.estimated_value_idr, OLD.next_action, OLD.next_follow_up_at,
    OLD.last_contact_at, OLD.closed_at
  );

  IF tracked_changed THEN
    IF NEW.last_activity_id IS NULL OR NEW.last_activity_id IS NOT DISTINCT FROM OLD.last_activity_id THEN
      RAISE EXCEPTION 'crm update requires a new timeline event'
        USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_update_requires_activity';
    END IF;
    SELECT * INTO event FROM crm_lead_activities
    WHERE id = NEW.last_activity_id AND lead_id = NEW.id AND operator_id = NEW.operator_id;
    IF NOT FOUND THEN
      RAISE EXCEPTION 'crm update activity does not match lead'
        USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_update_activity_scope';
    END IF;
    IF NEW.stage IS DISTINCT FROM OLD.stage THEN
      IF event.kind <> 'STAGE_CHANGED' OR event.from_stage IS DISTINCT FROM OLD.stage
         OR event.to_stage IS DISTINCT FROM NEW.stage THEN
        RAISE EXCEPTION 'crm stage transition does not match timeline event'
          USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_stage_activity_mismatch';
      END IF;
      IF NOT (
        (OLD.stage = 'NEW' AND NEW.stage IN ('CONTACT','OFFER','HOT','CLOSING','CANCELLED')) OR
        (OLD.stage = 'CONTACT' AND NEW.stage IN ('NEW','OFFER','HOT','CLOSING','CANCELLED')) OR
        (OLD.stage = 'OFFER' AND NEW.stage IN ('CONTACT','HOT','CLOSING','CANCELLED')) OR
        (OLD.stage = 'HOT' AND NEW.stage IN ('CONTACT','OFFER','CLOSING','CANCELLED')) OR
        (OLD.stage = 'CLOSING' AND NEW.stage IN ('CONTACT','CANCELLED')) OR
        (OLD.stage = 'CANCELLED' AND NEW.stage IN ('NEW','CONTACT'))
      ) THEN
        RAISE EXCEPTION 'invalid crm stage transition from % to %', OLD.stage, NEW.stage
          USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_stage_transition';
      END IF;
    ELSIF event.kind NOT IN ('PROFILE_UPDATED','CONTACT','NOTE','OFFER_SENT') THEN
      RAISE EXCEPTION 'crm profile update has incompatible timeline event'
        USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_profile_activity_mismatch';
    END IF;
  END IF;
  NEW.updated_at := NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER crm_leads_guard_update
  BEFORE UPDATE ON crm_leads
  FOR EACH ROW EXECUTE FUNCTION guard_crm_lead_update();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reject_crm_activity_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'crm lead activities are append-only'
    USING ERRCODE = 'check_violation', CONSTRAINT = 'crm_activities_append_only';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER crm_activities_append_only
  BEFORE UPDATE OR DELETE ON crm_lead_activities
  FOR EACH ROW EXECUTE FUNCTION reject_crm_activity_mutation();

-- +goose Down
DROP TRIGGER IF EXISTS crm_activities_append_only ON crm_lead_activities;
DROP FUNCTION IF EXISTS reject_crm_activity_mutation();
DROP TRIGGER IF EXISTS crm_leads_guard_update ON crm_leads;
DROP FUNCTION IF EXISTS guard_crm_lead_update();
DROP TRIGGER IF EXISTS crm_activities_relation_guard ON crm_lead_activities;
DROP FUNCTION IF EXISTS assert_crm_activity_relations();
DROP TRIGGER IF EXISTS crm_leads_relation_guard ON crm_leads;
DROP FUNCTION IF EXISTS assert_crm_lead_relations();
DROP TRIGGER IF EXISTS crm_leads_entitlement_guard ON crm_leads;
DROP FUNCTION IF EXISTS assert_crm_entitled();
DROP TRIGGER IF EXISTS crm_activities_branch_matches_operator ON crm_lead_activities;
DROP TRIGGER IF EXISTS crm_leads_branch_matches_operator ON crm_leads;
ALTER TABLE crm_leads DROP CONSTRAINT IF EXISTS crm_leads_last_activity_fkey;
DROP TABLE IF EXISTS crm_lead_activities;
DROP TABLE IF EXISTS crm_leads;
UPDATE plan_limits SET feature_flags = feature_flags - 'crm', updated_at = NOW();
