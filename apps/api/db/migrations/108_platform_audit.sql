-- +goose Up

-- Platform-level actions could not be audited at all, and were failing
-- silently: audit_logs.operator_id is NOT NULL with a foreign key, and a
-- platform action belongs to no tenant. Every attempt to record one — granting
-- platform access, reading somebody's identity number, changing a supplier —
-- was discarded by the error the caller ignored.
--
-- That is worse than having no audit trail, because the code claimed to have
-- one. It was found by a test asserting that reading an identity leaves a
-- trace; nothing else would have noticed.
ALTER TABLE audit_logs ALTER COLUMN operator_id DROP NOT NULL;

COMMENT ON COLUMN audit_logs.operator_id IS
  'NULL for platform-level actions, which belong to no tenant. Everything an operator does still carries theirs.';

-- entity_id was UUID, which platform actions cannot always satisfy: a Better
-- Auth account id is text, and granting somebody platform access is an action
-- about exactly that. Widened rather than worked around — encoding a text id
-- into a fake UUID to satisfy a column would make the trail unreadable.
ALTER TABLE audit_logs ALTER COLUMN entity_id TYPE TEXT;

-- Platform actions are the widest-privilege events in the system, so finding
-- them must not mean scanning every tenant's history.
CREATE INDEX audit_logs_platform_idx ON audit_logs (created_at DESC) WHERE operator_id IS NULL;

-- +goose Down
DROP INDEX audit_logs_platform_idx;
DELETE FROM audit_logs WHERE operator_id IS NULL;
ALTER TABLE audit_logs ALTER COLUMN entity_id TYPE UUID USING entity_id::uuid;
ALTER TABLE audit_logs ALTER COLUMN operator_id SET NOT NULL;
