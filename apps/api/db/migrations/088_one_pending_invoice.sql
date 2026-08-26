-- +goose Up
-- One unpaid invoice per operator, enforced by the database.
--
-- The service checked for an existing pending invoice and then inserted, which
-- two concurrent requests both pass — a double-clicked button or a retried
-- request leaves the operator holding two live unique amounts. Paying one
-- settles it while the other keeps its code reserved until it expires, and the
-- operator has two different figures on screen depending when they looked.
--
-- Uniqueness of this kind cannot be checked before inserting; only the database
-- can decide it. The service now returns the invoice that already exists when
-- this rejects a second one.
CREATE UNIQUE INDEX subscription_invoices_one_pending_idx
  ON subscription_invoices (operator_id)
  WHERE status = 'PENDING';

-- +goose Down
DROP INDEX IF EXISTS subscription_invoices_one_pending_idx;
