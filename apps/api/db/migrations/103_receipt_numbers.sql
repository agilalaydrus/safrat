-- +goose Up

-- A transaction had no number a person could quote. Orders are identified by
-- UUID, which is unusable over the phone and meaningless on a printed receipt —
-- and the owner asked for receipts every transacting account can preview and
-- print for themselves.
--
-- One global sequence rather than a counter per operator. A per-tenant counter
-- has to be read and incremented, which two concurrent checkouts race on, and
-- the usual fix is a lock that serialises every sale in the tenant. A sequence
-- never contends and never repeats. The cost is that numbers are not contiguous
-- within one travel — which nobody has asked for, and which would in any case
-- leak how many sales other tenants made.
CREATE SEQUENCE order_receipt_seq;

ALTER TABLE orders ADD COLUMN receipt_number TEXT NOT NULL
  DEFAULT ('INV-' || to_char(NOW(), 'YYYYMM') || '-' || lpad(nextval('order_receipt_seq')::text, 6, '0'));

-- Existing rows get numbers too: a receipt that cannot be printed for an older
-- transaction is a support call, not a feature gap.
UPDATE orders
SET receipt_number = 'INV-' || to_char(created_at, 'YYYYMM') || '-' || lpad(nextval('order_receipt_seq')::text, 6, '0')
WHERE receipt_number = '' OR receipt_number IS NULL;

ALTER TABLE orders ADD CONSTRAINT orders_receipt_number_key UNIQUE (receipt_number);

-- +goose Down
ALTER TABLE orders DROP CONSTRAINT orders_receipt_number_key;
ALTER TABLE orders DROP COLUMN receipt_number;
DROP SEQUENCE order_receipt_seq;
