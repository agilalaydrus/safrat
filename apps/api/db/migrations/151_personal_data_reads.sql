-- +goose Up
-- Who looked at somebody's personal data, and when.
--
-- Changes to personal data were already recorded; reads were not, except for
-- opening a KYC record. That gap grew the moment platform staff could look at a
-- tenant's own dashboard: a support session can page through every jamaah's
-- name, passport number and phone without leaving a single row behind.
--
-- One row per actor, per procedure, per day, per tenant — not one per request.
-- A row per request would be tens of thousands of rows nobody reads, and the
-- questions actually asked are "who has been looking at this tenant's jamaah
-- data" and "how much" — both answered by a count.
CREATE TABLE personal_data_reads (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- Better Auth "user".id. Text and without a foreign key, like every other
  -- record of who did something: it must outlive the account.
  actor_user_id   TEXT NOT NULL,
  -- Set when the read happened inside an impersonation session, so the two
  -- records can be read together. NULL for a platform-panel read.
  impersonation_id UUID REFERENCES impersonation_sessions(id) ON DELETE SET NULL,
  -- Whose data was read. NULL when the read is not scoped to one tenant.
  operator_id     UUID REFERENCES operators(id) ON DELETE CASCADE,
  -- The Connect procedure, e.g. /hajj.v1.PilgrimService/ListPilgrims.
  procedure       TEXT NOT NULL,
  -- Asia/Jakarta, matching every other daily boundary here. A day that ends at
  -- 07:00 local time would split an ordinary working evening in two.
  day             DATE NOT NULL,
  read_count      INTEGER NOT NULL DEFAULT 1 CHECK (read_count > 0),
  first_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The upsert key. A unique index rather than a primary key because two of its
-- columns are nullable, and NULLS NOT DISTINCT so that platform-panel reads
-- (no impersonation, no tenant) collapse onto one row per day instead of
-- silently inserting a new one on every request.
CREATE UNIQUE INDEX personal_data_reads_key_idx
  ON personal_data_reads (actor_user_id, procedure, day, impersonation_id, operator_id)
  NULLS NOT DISTINCT;

CREATE INDEX personal_data_reads_operator_idx ON personal_data_reads (operator_id, day DESC);
CREATE INDEX personal_data_reads_actor_idx ON personal_data_reads (actor_user_id, day DESC);

-- +goose Down
DROP TABLE IF EXISTS personal_data_reads;
