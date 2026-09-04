-- +goose Up
-- Moving a pilgrim from one package to another, after they have already paid.
--
-- "Pindah paket" is not a new order and not an edit of the old one — the old
-- order is what the money was actually received against, and rewriting its
-- price in place would make the payment history describe a transaction that
-- never happened. This is a record of the change itself: what it was, what it
-- became, and what that means for the difference already paid.
CREATE TABLE pilgrim_plan_changes (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id       UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id        UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  order_id          UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  from_product_id   UUID NOT NULL REFERENCES products(id),
  from_product_name TEXT NOT NULL,
  to_product_id     UUID NOT NULL REFERENCES products(id),
  to_product_name   TEXT NOT NULL,
  from_room_tier    TEXT,
  to_room_tier      TEXT,
  old_total_idr     BIGINT NOT NULL CHECK (old_total_idr >= 0),
  new_total_idr     BIGINT NOT NULL CHECK (new_total_idr >= 0),
  -- What had actually been received before the move, not the old package's
  -- list price — a pilgrim on an instalment plan may have paid less than
  -- old_total_idr, and the difference has to be measured against reality.
  paid_before_idr   BIGINT NOT NULL CHECK (paid_before_idr >= 0),
  -- Positive means the pilgrim now owes more; this table does not collect it,
  -- it only says how much and lets staff act through the payment flow that
  -- already exists.
  shortfall_idr     BIGINT NOT NULL DEFAULT 0 CHECK (shortfall_idr >= 0),
  -- Positive means the new package cost less than what was already paid.
  overpayment_idr   BIGINT NOT NULL DEFAULT 0 CHECK (overpayment_idr >= 0),
  CHECK (shortfall_idr = 0 OR overpayment_idr = 0),
  reason            TEXT NOT NULL CHECK (length(btrim(reason)) >= 10),
  changed_by        TEXT NOT NULL,
  idempotency_key   TEXT NOT NULL UNIQUE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX pilgrim_plan_changes_pilgrim_idx ON pilgrim_plan_changes (pilgrim_id, created_at DESC);
CREATE INDEX pilgrim_plan_changes_operator_idx ON pilgrim_plan_changes (operator_id, created_at DESC);

-- Kelebihan Bayar — the credit a pilgrim is owed, and what happens to it.
--
-- A separate table from the change record above because a credit has a
-- lifecycle of its own: it sits open until somebody either applies it to a
-- future order or refunds it, and that can happen long after the plan change
-- that created it.
CREATE TABLE pilgrim_credits (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id      UUID NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  amount_idr      BIGINT NOT NULL CHECK (amount_idr > 0),
  source          TEXT NOT NULL CHECK (source IN ('PLAN_CHANGE', 'MANUAL')),
  source_id       UUID,
  reason          TEXT NOT NULL CHECK (length(btrim(reason)) > 0),
  status          TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'APPLIED', 'REFUNDED')),
  applied_to_order_id UUID REFERENCES orders(id),
  applied_note    TEXT NOT NULL DEFAULT '',
  resolved_by     TEXT,
  resolved_at     TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK ((status = 'OPEN') = (resolved_at IS NULL)),
  CHECK (status <> 'APPLIED' OR applied_to_order_id IS NOT NULL)
);

CREATE INDEX pilgrim_credits_pilgrim_idx ON pilgrim_credits (pilgrim_id, status, created_at DESC);
-- Open credits are what a screen asks for most: "who is owed money right now".
CREATE INDEX pilgrim_credits_operator_open_idx ON pilgrim_credits (operator_id, created_at DESC) WHERE status = 'OPEN';

-- +goose Down
DROP TABLE IF EXISTS pilgrim_credits;
DROP TABLE IF EXISTS pilgrim_plan_changes;
