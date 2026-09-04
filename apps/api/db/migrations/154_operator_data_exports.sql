-- +goose Up
-- The portability right, not a screenshot of it.
--
-- UU PDP gives a data subject the right to receive their own data in a usable
-- form. This table is for the operator side of that: a travel agency asking
-- for everything TawafiqHub holds about their own business — jamaah, orders,
-- products, seasons — as one file they can keep, not a promise that support
-- will get to it eventually.
CREATE TABLE operator_data_exports (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id    UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- Better Auth "user".id, text like every other actor reference — the record
  -- of who asked for this must outlive their account.
  requested_by   TEXT NOT NULL,
  status         TEXT NOT NULL DEFAULT 'PENDING'
                 CHECK (status IN ('PENDING', 'PROCESSING', 'READY', 'FAILED')),
  -- Where the finished file lives, once it exists. Never a public URL — access
  -- is only ever a time-limited presigned link, generated on request.
  object_key     TEXT NOT NULL DEFAULT '',
  size_bytes     BIGINT NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  -- What went wrong, kept rather than discarded on failure — a support
  -- conversation about a failed export starts from this, not from asking the
  -- operator to reproduce it.
  error          TEXT NOT NULL DEFAULT '',
  requested_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at   TIMESTAMPTZ,
  -- A link that works forever is a link that outlives the reason it was
  -- issued. The file itself is deleted by the same sweep that expires it here
  -- — see the worker.
  expires_at     TIMESTAMPTZ,
  idempotency_key TEXT NOT NULL UNIQUE
);

CREATE INDEX operator_data_exports_operator_idx ON operator_data_exports (operator_id, requested_at DESC);
-- What the worker claims from: the oldest request nobody has started.
CREATE INDEX operator_data_exports_pending_idx ON operator_data_exports (requested_at)
  WHERE status = 'PENDING';

-- +goose Down
DROP TABLE IF EXISTS operator_data_exports;
