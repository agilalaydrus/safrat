-- +goose Up
-- Keep the database role vocabulary complete even though active dashboard
-- authorization is based on Better Auth membership plus branch_members.
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'BRANCH_HEAD';

-- +goose Down
-- user_role has no remaining column dependencies after migration 025 removed
-- the vestigial users table, so the enum can be rebuilt safely on rollback.
ALTER TYPE user_role RENAME TO user_role_with_branch_head;
CREATE TYPE user_role AS ENUM ('MANAGER', 'COORDINATOR', 'GROUP_LEADER');
DROP TYPE user_role_with_branch_head;
