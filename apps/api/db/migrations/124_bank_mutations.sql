-- +goose Up

-- Incoming bank credits, recorded before anything is decided about them.
--
-- Subscription invoices already carry an amount made unique to the rupiah, and
-- matching one was already possible — but only by a human reading a statement
-- and typing the figure. This is the feed: a bank API or a scraper posts what
-- arrived, and the matching happens without anybody watching.
--
-- Recorded first, matched second, and recorded even when nothing matches. A
-- mutation that matched nothing is the most important row in this table: it is
-- money that arrived and has not been accounted for, and it must be visible
-- rather than dropped because no invoice wanted it.
CREATE TABLE bank_mutations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  -- The bank's own identifier for the entry. The whole idempotency story:
  -- feeds redeliver, scrapers re-read the same page, and a retry must not
  -- settle an invoice twice.
  external_id TEXT NOT NULL,

  -- API, SCRAPER, or MANUAL. Kept because they carry different weight — a
  -- scraped row is a best-effort reading of a web page, a MANUAL one is
  -- somebody's typing, and neither should be mistaken for a bank's own API.
  source TEXT NOT NULL CHECK (source IN ('API', 'SCRAPER', 'MANUAL')),

  amount_idr BIGINT NOT NULL CHECK (amount_idr > 0),
  -- What the bank called it. Useless for matching and essential for a dispute.
  description TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  -- UNMATCHED until an invoice claims it. IGNORED is a person saying this
  -- credit is not a subscription payment at all, which is a decision worth
  -- recording rather than leaving the row to sit forever.
  status TEXT NOT NULL DEFAULT 'UNMATCHED'
    CHECK (status IN ('UNMATCHED', 'MATCHED', 'IGNORED')),
  matched_invoice_id UUID REFERENCES subscription_invoices(id) ON DELETE RESTRICT,
  matched_at TIMESTAMPTZ,
  matched_by_user_id TEXT,
  note TEXT NOT NULL DEFAULT '',

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  -- Status and the invoice cannot disagree. Without this a row could claim to
  -- be matched while pointing at nothing, and reconciliation would be arguing
  -- with itself.
  CONSTRAINT bank_mutations_matched_has_invoice_check
    CHECK ((status = 'MATCHED') = (matched_invoice_id IS NOT NULL))
);

-- Redelivery is a no-op rather than a second settlement. Per source, because
-- two feeds legitimately number their own entries independently.
CREATE UNIQUE INDEX bank_mutations_source_external_idx
  ON bank_mutations (source, external_id);

-- One credit settles at most one invoice. A bank entry claimed by two invoices
-- would mean money counted twice, and no later report could tell.
CREATE UNIQUE INDEX bank_mutations_invoice_idx
  ON bank_mutations (matched_invoice_id) WHERE matched_invoice_id IS NOT NULL;

-- The queue a person works: money in, unaccounted for, oldest first.
CREATE INDEX bank_mutations_unmatched_idx
  ON bank_mutations (occurred_at) WHERE status = 'UNMATCHED';

CREATE TRIGGER bank_mutations_set_updated_at
  BEFORE UPDATE ON bank_mutations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A recorded credit is evidence of money arriving. The application never
-- deletes one — a mutation that turns out not to be a payment is IGNORED, not
-- erased, so the record of it arriving survives the decision about it.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE DELETE, TRUNCATE ON bank_mutations FROM safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS bank_mutations;
