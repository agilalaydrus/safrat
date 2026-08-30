-- +goose Up

-- Passport numbers are encrypted at rest.
--
-- docs/INSIDEN-DATA-PRIBADI.md says plainly what a database dump exposes today:
-- the name, passport number, date of birth, phone, email and address of every
-- jamaah. Of those, the passport number is the one that enables impersonation
-- rather than merely embarrassment, and it is the one this closes.
--
-- I had written that these columns could not be encrypted because they are
-- searched and sorted. That is true of full_name and false of this one:
-- passport_number is only ever matched exactly — one query, no LIKE, no ORDER
-- BY — so a keyed blind index restores the single lookup that needs it.
--
-- The column keeps its name and holds either form during the migration. The
-- sealer passes plaintext through unchanged, which is what lets a half-migrated
-- table stay readable instead of needing a stop-the-world backfill.
ALTER TABLE pilgrims
  -- HMAC of the normalised number, keyed from the encryption key. Deterministic
  -- so the lookup works; keyed so a stolen database cannot be brute-forced by
  -- hashing candidate numbers, of which there are far too few to resist a plain
  -- hash.
  ADD COLUMN passport_number_blind TEXT NOT NULL DEFAULT '',
  -- Which key sealed it. A record that will not open is almost always the wrong
  -- key, and this says which one was used without revealing it.
  ADD COLUMN passport_key_fingerprint TEXT NOT NULL DEFAULT '';

-- The lookup index. Partial, because rows not yet migrated carry no token and
-- there is no reason to index the gap.
CREATE INDEX pilgrims_passport_blind_idx
  ON pilgrims (operator_id, passport_number_blind)
  WHERE passport_number_blind <> '';

-- Ciphertext and stamp move together or not at all. A row claiming a key it was
-- not sealed with is unopenable and undiagnosable, and the mismatch would only
-- surface when somebody needed the number.
ALTER TABLE pilgrims ADD CONSTRAINT pilgrims_passport_stamp_consistent_check
  CHECK ((passport_number_blind = '') = (passport_key_fingerprint = ''));

-- Deliberately not done here: no backfill, and no NOT NULL on the new columns.
-- Encrypting existing rows needs the key, which lives in the application's
-- environment and not in the database — the same reason migration 106 left KYC
-- identity numbers to a Go migrator. cmd/rotatekyc does that work; this only
-- makes room for it.

-- +goose Down
DROP INDEX IF EXISTS pilgrims_passport_blind_idx;
ALTER TABLE pilgrims
  DROP CONSTRAINT IF EXISTS pilgrims_passport_stamp_consistent_check,
  DROP COLUMN IF EXISTS passport_key_fingerprint,
  DROP COLUMN IF EXISTS passport_number_blind;
