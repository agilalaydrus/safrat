-- +goose Up
-- A branch member must be recorded under the same operator that owns the
-- branch. The separate foreign keys created in migration 128 only prove that
-- both IDs exist; they do not prove that they belong together.

ALTER TABLE branches
  ADD CONSTRAINT branches_id_operator_key UNIQUE (id, operator_id);

ALTER TABLE branch_members
  ADD CONSTRAINT branch_members_branch_operator_fkey
  FOREIGN KEY (branch_id, operator_id)
  REFERENCES branches (id, operator_id)
  ON DELETE CASCADE;

-- +goose Down
ALTER TABLE branch_members
  DROP CONSTRAINT IF EXISTS branch_members_branch_operator_fkey;

ALTER TABLE branches
  DROP CONSTRAINT IF EXISTS branches_id_operator_key;
