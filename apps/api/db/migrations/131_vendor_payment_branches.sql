-- +goose Up
-- Vendor obligations can be owned by a branch as well as by head office.
-- Existing rows intentionally remain NULL: they predate branch accounting and
-- therefore remain central obligations until deliberately allocated.
ALTER TABLE vendor_payments
  ADD COLUMN branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT;

CREATE INDEX vendor_payments_branch_idx
  ON vendor_payments (branch_id) WHERE branch_id IS NOT NULL;

CREATE TRIGGER vendor_payments_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON vendor_payments
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();

-- +goose Down
DROP TRIGGER IF EXISTS vendor_payments_branch_matches_operator ON vendor_payments;
DROP INDEX IF EXISTS vendor_payments_branch_idx;
ALTER TABLE vendor_payments DROP COLUMN IF EXISTS branch_id;
