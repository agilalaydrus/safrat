-- +goose Up
-- groups.leader_id previously referenced the vestigial `users` (plural) table
-- left over from a pre-Better-Auth "Clerk" migration (see 003_rename_clerk_auth_columns.sql)
-- — a UUID-keyed table that was never populated (0 rows) and is unrelated to the
-- real Better Auth identity table, "user" (singular, text id), which every
-- session/member/account row actually references. groups.leader_id must point
-- at "user" to mean anything, so fix the column type and FK to match.
ALTER TABLE groups DROP CONSTRAINT groups_leader_id_fkey;
ALTER TABLE groups ALTER COLUMN leader_id TYPE TEXT USING leader_id::text;
ALTER TABLE groups ADD CONSTRAINT groups_leader_id_fkey
  FOREIGN KEY (leader_id) REFERENCES "user"(id) ON DELETE SET NULL;
-- `users` (plural) was that vestigial Clerk-era table — 0 rows, and after the
-- fix above nothing references it. Dropping it also avoids a Go codegen name
-- collision: sqlc singularizes `users` to the same `User` type as `"user"`.
DROP TABLE users;

-- +goose Down
CREATE TABLE users (
  id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  better_auth_user_id  TEXT        NOT NULL,
  operator_id          UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  role                 user_role   NOT NULL,
  name                 TEXT        NOT NULL,
  email                TEXT        NOT NULL,
  phone                TEXT,
  avatar_url           TEXT,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT users_clerk_id_key UNIQUE (better_auth_user_id)
);
CREATE INDEX users_operator_id_idx ON users(operator_id);
ALTER TABLE groups DROP CONSTRAINT groups_leader_id_fkey;
ALTER TABLE groups ALTER COLUMN leader_id TYPE UUID USING NULL;
ALTER TABLE groups ADD CONSTRAINT groups_leader_id_fkey
  FOREIGN KEY (leader_id) REFERENCES users(id) ON DELETE SET NULL;
