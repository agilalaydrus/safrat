-- +goose Up
-- Lifting a suspension is its own privileged action, not a SUSPEND row with a
-- flag buried in its payload.
--
-- The two are asked about separately — "why was this travel agency locked out"
-- and "who let them back in" — and a payload field cannot be indexed, grouped
-- or read at a glance. The kinds were declared in 138 before either action
-- existed; this adds the one that was missing.
ALTER TABLE privileged_actions DROP CONSTRAINT privileged_actions_kind_check;
ALTER TABLE privileged_actions ADD CONSTRAINT privileged_actions_kind_check
  CHECK (kind IN ('SUSPEND','REINSTATE','DELETE_TENANT','SET_PLAN_LIMIT','SET_SETTLEMENT'));

-- +goose Down
DELETE FROM privileged_actions WHERE kind = 'REINSTATE';
ALTER TABLE privileged_actions DROP CONSTRAINT privileged_actions_kind_check;
ALTER TABLE privileged_actions ADD CONSTRAINT privileged_actions_kind_check
  CHECK (kind IN ('SUSPEND','DELETE_TENANT','SET_PLAN_LIMIT','SET_SETTLEMENT'));
