-- +goose Up
-- Looking at a tenant's dashboard as they see it, with a record of every time.
--
-- Support cannot reproduce "the button does nothing" from a list of database
-- rows. The alternative people reach for is asking the customer for their
-- password, which is worse in every way and leaves no trace at all.
--
-- Three things make this safe rather than a master key:
--
--  1. It is read-only. Not "we avoid writing" — the interceptor refuses every
--     procedure that is not a read, so a write cannot be issued at all. Fixing
--     something for a customer goes through the platform RPCs, which have their
--     own confirmation and their own audit trail. Never by wearing their face.
--  2. It expires. A session that lasts until somebody remembers to close it is
--     a session that never closes.
--  3. It is written down before it starts, with a reason, and the row survives
--     the session ending.
CREATE TABLE impersonation_sessions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- Better Auth's "user".id, text like every other reference to it. No FK: the
  -- record of who did this must outlive the account that did it.
  admin_user_id   TEXT NOT NULL,
  operator_id     UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- SHA-256 of the token, never the token. A leaked database must not hand
  -- somebody a working impersonation session.
  token_hash      TEXT NOT NULL UNIQUE CHECK (length(token_hash) = 64),
  -- Long enough to be a sentence. "test" is not a reason, and this column is
  -- the only thing that will explain the session a year from now.
  reason          TEXT NOT NULL CHECK (length(btrim(reason)) >= 10),
  ip              TEXT NOT NULL DEFAULT '',
  user_agent      TEXT NOT NULL DEFAULT '',
  started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at      TIMESTAMPTZ NOT NULL,
  ended_at        TIMESTAMPTZ,
  ended_reason    TEXT NOT NULL DEFAULT '',
  -- A double-clicked button must not open two sessions. Unique in the database
  -- rather than checked first and inserted after: two concurrent requests both
  -- pass a check-then-act.
  idempotency_key TEXT NOT NULL UNIQUE,
  CHECK (expires_at > started_at)
);

-- The lookup on every impersonated request: by hash, still open, not expired.
CREATE INDEX impersonation_sessions_live_idx
  ON impersonation_sessions (token_hash)
  WHERE ended_at IS NULL;

-- "What has this admin been doing" and "who has been inside this tenant".
CREATE INDEX impersonation_sessions_admin_idx ON impersonation_sessions (admin_user_id, started_at DESC);
CREATE INDEX impersonation_sessions_operator_idx ON impersonation_sessions (operator_id, started_at DESC);

-- +goose Down
DROP TABLE IF EXISTS impersonation_sessions;
