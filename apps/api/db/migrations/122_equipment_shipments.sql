-- +goose Up

-- Equipment was sold and there was nowhere to record delivering it. A paid
-- EQUIPMENT order opened a fulfilment like any other, and then:
--
--   the fast path found no supplier route, so it sat PENDING forever;
--   the sweep set NEEDS_REVIEW with "Produk belum punya routing supplier aktif"
--
-- The second is worse than the first. It puts a parcel into the supplier queue
-- under a fault that can never be fixed, because equipment has no supplier to
-- route to — so the queue that exists to be emptied fills with things nobody
-- can act on, and the real routing failures get lost among them.
--
-- Money had already been taken in both cases, with no address, no courier and
-- no record of a handover.

-- Two kinds of delivery, told apart explicitly rather than inferred from
-- whether supplier_id happens to be NULL. Inference is how an equipment order
-- ended up in the supplier queue in the first place.
ALTER TABLE order_fulfilments
  ADD COLUMN kind TEXT NOT NULL DEFAULT 'SUPPLIER'
    CHECK (kind IN ('SUPPLIER', 'SHIPMENT'));

-- SHIP or PICKUP. A jamaah collecting a bag at the travel's office is the
-- normal case here, not an edge one, and forcing an address on it would mean
-- staff inventing one.
ALTER TABLE order_fulfilments
  ADD COLUMN delivery_method TEXT NOT NULL DEFAULT ''
    CHECK (delivery_method IN ('', 'SHIP', 'PICKUP')),

  -- Where it goes. Captured on the fulfilment rather than frozen at checkout,
  -- because equipment is routinely bought without an address — the operator
  -- confirms it by phone afterwards. It stops being editable at dispatch (see
  -- the trigger below), which is the moment that actually matters.
  ADD COLUMN recipient_name TEXT NOT NULL DEFAULT '',
  ADD COLUMN recipient_phone TEXT NOT NULL DEFAULT '',
  ADD COLUMN shipping_address TEXT NOT NULL DEFAULT '',

  -- How it travelled. Both required before a shipment can be marked sent: a
  -- parcel with no tracking is one nobody can answer a question about.
  ADD COLUMN courier TEXT NOT NULL DEFAULT '',
  ADD COLUMN tracking_number TEXT NOT NULL DEFAULT '',

  -- Proof it arrived. Who signed for it, and who on staff recorded that.
  -- Separate people on purpose: the recipient is what a dispute is argued
  -- with, the staff id is who to ask about it.
  ADD COLUMN handover_recipient TEXT NOT NULL DEFAULT '',
  ADD COLUMN handover_note TEXT NOT NULL DEFAULT '',
  ADD COLUMN handed_over_by_user_id TEXT;

-- A shipment has no supplier, and a supplier fulfilment has no address. Left
-- to convention these blur, and a half-filled row is unreadable six months
-- later when somebody is trying to work out what happened.
ALTER TABLE order_fulfilments ADD CONSTRAINT order_fulfilments_kind_fields_check
  CHECK (
    (kind = 'SUPPLIER' AND delivery_method = '' AND shipping_address = ''
      AND courier = '' AND tracking_number = '' AND handover_recipient = '')
    OR
    (kind = 'SHIPMENT' AND supplier_id IS NULL AND supplier_reference = '')
  );

-- A posted parcel must carry courier and tracking; a collected one must not.
-- Enforced at the row rather than in the service because this is what a
-- customer is told when they ask where their order is.
ALTER TABLE order_fulfilments ADD CONSTRAINT order_fulfilments_shipment_sent_check
  CHECK (
    kind <> 'SHIPMENT' OR status NOT IN ('SENT', 'DELIVERED')
    OR (delivery_method = 'SHIP' AND courier <> '' AND tracking_number <> ''
        AND shipping_address <> '' AND recipient_name <> '')
    OR (delivery_method = 'PICKUP')
  );

-- Delivered means somebody received it. Without this, "delivered" is a status
-- a person clicked, which is exactly what it must not be for goods.
ALTER TABLE order_fulfilments ADD CONSTRAINT order_fulfilments_shipment_delivered_check
  CHECK (
    kind <> 'SHIPMENT' OR status <> 'DELIVERED'
    OR (handover_recipient <> '' AND handed_over_by_user_id IS NOT NULL
        AND delivered_at IS NOT NULL)
  );

-- Where a parcel was sent stops being editable once it has been sent. Before
-- dispatch an operator correcting an address read over the phone is ordinary
-- work; after it, changing the address rewrites the answer to "where did this
-- go?" and there is no way to tell it was ever different.
-- +goose StatementBegin
CREATE FUNCTION freeze_dispatched_shipment() RETURNS trigger AS $$
BEGIN
  IF OLD.kind = 'SHIPMENT' AND OLD.status IN ('SENT', 'DELIVERED') THEN
    IF NEW.shipping_address IS DISTINCT FROM OLD.shipping_address
       OR NEW.recipient_name IS DISTINCT FROM OLD.recipient_name
       OR NEW.recipient_phone IS DISTINCT FROM OLD.recipient_phone
       OR NEW.delivery_method IS DISTINCT FROM OLD.delivery_method
       OR NEW.courier IS DISTINCT FROM OLD.courier
       OR NEW.tracking_number IS DISTINCT FROM OLD.tracking_number THEN
      RAISE EXCEPTION 'pengiriman % sudah berjalan; tujuan dan resi tidak dapat diubah', OLD.order_id
        USING ERRCODE = 'check_violation';
    END IF;
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER order_fulfilments_freeze_dispatched
  BEFORE UPDATE ON order_fulfilments
  FOR EACH ROW EXECUTE FUNCTION freeze_dispatched_shipment();

-- Existing equipment fulfilments are re-labelled and taken out of the supplier
-- queue. Their NEEDS_REVIEW was never a supplier fault, and leaving them there
-- would keep the queue unusable.
UPDATE order_fulfilments f
SET kind = 'SHIPMENT',
    status = CASE WHEN f.status IN ('NEEDS_REVIEW', 'PENDING') THEN 'PENDING' ELSE f.status END,
    last_error = CASE WHEN f.last_error = 'Produk belum punya routing supplier aktif'
                      THEN '' ELSE f.last_error END,
    supplier_id = NULL,
    supplier_reference = ''
FROM orders o
JOIN products p ON p.id = o.product_id
WHERE o.id = f.order_id AND p.category = 'EQUIPMENT';

-- The queue an operator works from: parcels owed, oldest first.
CREATE INDEX order_fulfilments_shipment_queue_idx
  ON order_fulfilments (created_at) WHERE kind = 'SHIPMENT' AND status <> 'DELIVERED';

-- +goose Down
DROP INDEX IF EXISTS order_fulfilments_shipment_queue_idx;
DROP TRIGGER IF EXISTS order_fulfilments_freeze_dispatched ON order_fulfilments;
DROP FUNCTION IF EXISTS freeze_dispatched_shipment();
ALTER TABLE order_fulfilments
  DROP CONSTRAINT IF EXISTS order_fulfilments_shipment_delivered_check,
  DROP CONSTRAINT IF EXISTS order_fulfilments_shipment_sent_check,
  DROP CONSTRAINT IF EXISTS order_fulfilments_kind_fields_check,
  DROP COLUMN IF EXISTS handed_over_by_user_id,
  DROP COLUMN IF EXISTS handover_note,
  DROP COLUMN IF EXISTS handover_recipient,
  DROP COLUMN IF EXISTS tracking_number,
  DROP COLUMN IF EXISTS courier,
  DROP COLUMN IF EXISTS shipping_address,
  DROP COLUMN IF EXISTS recipient_phone,
  DROP COLUMN IF EXISTS recipient_name,
  DROP COLUMN IF EXISTS delivery_method,
  DROP COLUMN IF EXISTS kind;
