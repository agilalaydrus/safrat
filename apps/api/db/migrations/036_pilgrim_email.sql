-- +goose Up
-- Enables pilgrim account access "the same as admin/leader" — sign up or
-- sign in with a real email at the shared /sign-in, no app_access_code
-- link required. A Better Auth session's email is matched against this
-- column (see lib/auth.ts's session.create.after hook) to auto-set
-- pilgrims.linked_user_id — unique so that match is never ambiguous.
ALTER TABLE pilgrims ADD COLUMN email TEXT UNIQUE;

-- +goose Down
ALTER TABLE pilgrims DROP COLUMN IF EXISTS email;
