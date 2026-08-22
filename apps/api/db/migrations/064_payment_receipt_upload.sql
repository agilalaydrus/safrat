-- +goose Up
-- payment_receipt_url was a free-text field the admin had to paste a link
-- into — in practice a payment proof is a photo/screenshot of a bank
-- transfer, so it belongs in pilgrim_documents (real file upload, camera
-- capture) like every other document, not a manually-typed URL. A pilgrim
-- can also pay in installments (DP then pelunasan), each with its own
-- receipt — pilgrim_documents already supports multiple files per type.
ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','KTP','SELFIE','KK','MAHRAM_PROOF','VISA','PAYMENT_RECEIPT','OTHER'));

ALTER TABLE pilgrims DROP COLUMN IF EXISTS payment_receipt_url;

-- +goose Down
ALTER TABLE pilgrims ADD COLUMN IF NOT EXISTS payment_receipt_url TEXT NOT NULL DEFAULT '';

ALTER TABLE pilgrim_documents DROP CONSTRAINT IF EXISTS pilgrim_documents_doc_type_check;
ALTER TABLE pilgrim_documents ADD CONSTRAINT pilgrim_documents_doc_type_check
  CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','KTP','SELFIE','KK','MAHRAM_PROOF','VISA','OTHER'));
