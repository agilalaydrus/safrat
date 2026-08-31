-- +goose Up
-- A branch has one accountable head. The primary key already prevents one
-- person from heading two branches; this closes the inverse race as well.
CREATE UNIQUE INDEX branch_members_one_head_idx ON branch_members (branch_id);

-- +goose Down
DROP INDEX IF EXISTS branch_members_one_head_idx;
