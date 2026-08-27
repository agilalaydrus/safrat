-- +goose Up

-- The payment window is a day, because scanning a QRIS code or carrying a
-- virtual account number to a bank is not instant. The poller that guards
-- against a lost webhook would otherwise ask the gateway about every unpaid
-- order every two minutes for that whole day — around seven hundred calls per
-- abandoned checkout, across every tenant.
--
-- That is its own kind of loss: exhausting the gateway's rate limit would stop
-- settlement working for everybody, which is far worse than the dropped webhook
-- the poller exists to catch.
--
-- So checks back off as an order ages. Payment usually happens in the first
-- minutes, so that is where the attention goes; an order still unpaid after two
-- hours is probably abandoned and is checked hourly until its invoice expires.
ALTER TABLE orders ADD COLUMN last_gateway_check_at TIMESTAMPTZ;

-- Partial: only pending orders are ever polled, and this index is what keeps
-- the sweep from scanning the whole table as the order history grows.
CREATE INDEX orders_gateway_check_idx
  ON orders (last_gateway_check_at NULLS FIRST)
  WHERE status = 'PENDING' AND xendit_invoice_id IS NOT NULL;

-- +goose Down
DROP INDEX orders_gateway_check_idx;
ALTER TABLE orders DROP COLUMN last_gateway_check_at;
