-- +goose Up
-- Leaders automatically become agents (can sell any of the operator's
-- products, per the referral/commission system already in place) —
-- linked_user_id ties an agent row back to the real Better Auth identity
-- so GroupService.UpdateGroup can create-or-find idempotently instead of
-- risking a duplicate agent row every time a leader is reassigned.
ALTER TABLE agents ADD COLUMN linked_user_id TEXT UNIQUE REFERENCES "user"(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE agents DROP COLUMN IF EXISTS linked_user_id;
