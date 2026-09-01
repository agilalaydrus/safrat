-- +goose Up

-- A tenant-wide audit trail can itself leak another branch's personal data:
-- descriptions include actions about pilgrims, payments, health and refunds.
-- Store the actor's branch as a historical fact when the row is appended.
-- It must not be derived when reading, because moving a staff member later
-- must not silently move their old audit history to the new branch.
ALTER TABLE audit_logs
  ADD COLUMN branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT;

CREATE INDEX audit_logs_branch_created_idx
  ON audit_logs (branch_id, created_at DESC) WHERE branch_id IS NOT NULL;

-- Reuse the tenant-integrity trigger introduced with branches. Application
-- inserts derive this value from branch_members, but the database still
-- refuses a mismatched pair from maintenance SQL or a future code path.
CREATE TRIGGER audit_logs_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();

-- Existing logs intentionally remain NULL. We cannot know whether their actor
-- belonged to the same branch at the historical event time; backfilling from
-- today's membership would manufacture evidence. They remain visible to head
-- office and hidden from branch-scoped readers.

-- +goose Down
DROP TRIGGER IF EXISTS audit_logs_branch_matches_operator ON audit_logs;
DROP INDEX IF EXISTS audit_logs_branch_created_idx;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS branch_id;
