-- +goose Up
-- Links a pilgrim record to a real Better Auth identity once they sign in
-- with Google from their /pilgrim/[code] link. Nullable — most pilgrims may
-- never link (the access-code link keeps working either way); this exists
-- so a verified identity is available for future money-touching actions
-- (Module 7 orders/payments) instead of trusting the access code alone.
ALTER TABLE pilgrims
  ADD COLUMN linked_user_id TEXT UNIQUE REFERENCES "user"(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE pilgrims DROP COLUMN IF EXISTS linked_user_id;
