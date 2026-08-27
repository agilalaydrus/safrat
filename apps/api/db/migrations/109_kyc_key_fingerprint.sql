-- +goose Up

-- Which key sealed this record.
--
-- Eight hex characters of a SHA-256 over the key — it identifies a key without
-- revealing it, and reversing it would mean breaking SHA-256. Safe to store
-- beside the data it describes, and safe to read out over the phone.
--
-- Two things become possible with it that were not. A record sealed with a key
-- nobody has any more can be *identified* rather than merely failing to open,
-- so somebody looking for the right key knows what they are looking for. And a
-- rotation can proceed record by record, because which key each row needs is
-- written on the row.
ALTER TABLE kyc_records ADD COLUMN key_fingerprint TEXT NOT NULL DEFAULT '';

CREATE INDEX kyc_records_key_fingerprint_idx
  ON kyc_records (key_fingerprint) WHERE key_fingerprint <> '';

-- +goose Down
DROP INDEX kyc_records_key_fingerprint_idx;
ALTER TABLE kyc_records DROP COLUMN key_fingerprint;
