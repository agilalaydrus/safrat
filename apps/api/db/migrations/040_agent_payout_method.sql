-- +goose Up
-- Structured, not just free text in `note` — lets the ledger answer "how did
-- this money move" without parsing prose, and gives the payout dialog a
-- real select instead of asking the operator to type it consistently.
ALTER TABLE agent_payouts
  ADD COLUMN method TEXT NOT NULL DEFAULT 'TRANSFER' CHECK (method IN ('TRANSFER', 'CASH', 'EWALLET'));

-- +goose Down
ALTER TABLE agent_payouts DROP COLUMN IF EXISTS method;
