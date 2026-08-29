-- +goose Up

-- A handover was documented by a name typed into a box. That is a real record
-- and it is also the weakest kind: nothing distinguishes a name somebody wrote
-- because the jamaah signed for the parcel from a name somebody wrote because
-- the queue needed clearing.
--
-- The photo is the part that is hard to invent. Stored privately — a delivery
-- receipt shows a person's name, their signature and often their doorway, none
-- of which belongs on a public asset URL — and reached only through a
-- short-lived signed link.
--
-- The key, not the image. Postgres is the wrong place for photographs, and a
-- database dump should not turn into a folder of people's front doors.
ALTER TABLE order_fulfilments
  ADD COLUMN handover_proof_key TEXT NOT NULL DEFAULT '';

-- Proof belongs to a handover. Without this a key could sit on a row that never
-- recorded one, which would read as evidence of something that did not happen.
ALTER TABLE order_fulfilments ADD CONSTRAINT order_fulfilments_proof_needs_handover_check
  CHECK (handover_proof_key = '' OR handover_recipient <> '');

-- +goose Down
ALTER TABLE order_fulfilments
  DROP CONSTRAINT IF EXISTS order_fulfilments_proof_needs_handover_check,
  DROP COLUMN IF EXISTS handover_proof_key;
