-- +goose Up
-- What each tenant is actually using, measured once a day.
--
-- Plan limits have been enforced since migration 132, but nothing showed who
-- was approaching one. A travel agency hitting its ceiling mid-season finds out
-- when a registration is refused — after the fact, and from the wrong side.
-- The point of measuring is to offer an upgrade before that happens.
--
-- Computed by a daily worker rather than on read. Counting pilgrims, branches
-- and stored bytes across every tenant on each panel load would be the most
-- expensive query in the system, and it would get slower exactly as the
-- business grew.

CREATE TABLE usage_counters (
  operator_id  UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- pilgrims | branches | storage_bytes.
  --
  -- Deliberately only what can be measured today. API calls and WhatsApp
  -- messages are named in the design, but nothing counts either one yet — an
  -- api_calls row reading zero would be indistinguishable from a tenant making
  -- no calls, and somebody would eventually make a decision on it.
  metric       TEXT NOT NULL CHECK (metric IN ('pilgrims', 'branches', 'storage_bytes')),
  -- First day of the period. Daily snapshots, so the primary key makes a
  -- second run for the same day overwrite rather than duplicate.
  period_start DATE NOT NULL,
  value        BIGINT NOT NULL CHECK (value >= 0),
  computed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (operator_id, metric, period_start)
);

-- The panel reads the latest day across every tenant; this keeps that cheap.
CREATE INDEX usage_counters_period_idx ON usage_counters (period_start DESC, metric);

-- +goose Down
DROP TABLE IF EXISTS usage_counters;
