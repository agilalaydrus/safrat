-- +goose Up

-- TawafiqHub itself has no identity in this system. Every user belongs to an
-- operator, and operator staff are deliberately confined to their own tenant —
-- correctly, but it leaves platform work with nowhere to happen except a
-- terminal on the VPS.
--
-- An explicit table rather than a flag on Better Auth's "user": Better Auth
-- owns and migrates that table, so a column added here would be outside its
-- schema management. More importantly, platform access is the widest privilege
-- in the system and should be a row somebody has to deliberately insert, not
-- a boolean that could be flipped by any code path touching a user record.
--
-- There is no self-service path to this table on purpose. It is granted by
-- someone with database access, which is the point: the panel it unlocks is
-- what removes the need for database access for everything else.
CREATE TABLE platform_admins (
  -- Better Auth's "user"(id) is TEXT. No FK, for the same reason the rest of
  -- the schema avoids one against a table another system migrates.
  user_id TEXT PRIMARY KEY,
  -- Recorded so a revoked admin's history is still explicable.
  note TEXT NOT NULL DEFAULT '',
  granted_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE platform_admins;
