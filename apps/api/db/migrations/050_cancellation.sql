-- +goose Up

-- Operator-defined refund policy for a season. Tiers are evaluated in
-- sort_order ASC; the first tier where min_days <= days_before_departure
-- wins (e.g. a ">=90 days: 100%" tier must sort before ">=60 days: 75%"
-- so a cancellation 95 days out matches the 100% tier, not the 75% one).
CREATE TABLE cancellation_policies (
  id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID          NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID          NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  name          TEXT          NOT NULL,
  min_days      INTEGER       NOT NULL,
  refund_pct    FLOAT8        NOT NULL CHECK (refund_pct BETWEEN 0 AND 100),
  sort_order    INTEGER       NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX cancellation_policies_season_idx ON cancellation_policies(season_id, sort_order);
COMMENT ON TABLE cancellation_policies IS
  'Refund tiers per season. Evaluated in sort_order ASC. First tier where min_days <= days_before wins.';

-- Immutable cancellation record — never UPDATEd after INSERT. total_paid
-- and refund_amount are snapshots computed server-side from the pilgrim's
-- PAID orders at the moment of cancellation (see CancellationRepository),
-- not a running balance column, so a later order never rewrites history.
-- BIGINT rupiah, matching orders.total_price_idr — this codebase never
-- represents IDR with fractional subunits.
CREATE TABLE pilgrim_cancellations (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id      UUID        NOT NULL REFERENCES pilgrims(id),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id       UUID        NOT NULL REFERENCES seasons(id),
  reason          TEXT        NOT NULL DEFAULT '',
  days_before     INTEGER     NOT NULL,
  refund_pct      FLOAT8      NOT NULL,
  refund_amount_idr BIGINT    NOT NULL DEFAULT 0,
  total_paid_idr  BIGINT      NOT NULL DEFAULT 0,
  cancelled_by    TEXT        NOT NULL,
  cancelled_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  policy_id       UUID        REFERENCES cancellation_policies(id) ON DELETE SET NULL,
  UNIQUE (pilgrim_id)
);

CREATE INDEX pilgrim_cancellations_operator_season_idx ON pilgrim_cancellations(operator_id, season_id);

-- Lifecycle status alongside the existing is_substituted flag — that flag
-- means "this slot was handed to someone else", this means "this person
-- is no longer coming at all". The two are independent: a cancelled
-- pilgrim is never substituted (no replacement), so no third combined
-- enum value is needed.
ALTER TABLE pilgrims
  ADD COLUMN status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','CANCELLED'));

-- +goose Down
DROP TABLE IF EXISTS pilgrim_cancellations;
DROP TABLE IF EXISTS cancellation_policies;
ALTER TABLE pilgrims DROP COLUMN IF EXISTS status;
