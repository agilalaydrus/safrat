-- +goose Up
ALTER TABLE operators RENAME COLUMN clerk_org_id TO better_auth_org_id;
ALTER TABLE users RENAME COLUMN clerk_id TO better_auth_user_id;

-- +goose Down
ALTER TABLE users RENAME COLUMN better_auth_user_id TO clerk_id;
ALTER TABLE operators RENAME COLUMN better_auth_org_id TO clerk_org_id;
