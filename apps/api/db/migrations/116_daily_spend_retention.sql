-- +goose Up

-- Daily spend is kept for three days (owner's decision), then removed. The
-- limit itself only ever needs today; the extra days are so a disputed
-- transaction can still be reconciled against the day it was made, and so a
-- reversal arriving late has something to decrement.
--
-- The awkward part is who is allowed to do the deleting. Migration 115 took
-- DELETE on this table away from the application role, because removing a row
-- hands an account its whole daily limit back — and that reasoning does not
-- stop being true just because a purge needs to exist.
--
-- So the purge is a SECURITY DEFINER function with the cutoff built into it,
-- rather than a privilege. The application can ask for expired rows to be
-- removed and cannot remove anything else: today's row is unreachable through
-- this, which is exactly the row that would be worth attacking.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION purge_daily_digital_spend(keep_days INT DEFAULT 3)
RETURNS INT
LANGUAGE plpgsql
SECURITY DEFINER
-- Empty search_path so a caller cannot shadow a table name and have this
-- delete from something else. A definer function without it is a way to run
-- code as its owner.
SET search_path = pg_catalog, public
AS $$
DECLARE
  removed INT;
  cutoff DATE;
BEGIN
  -- Clamped, not trusted. A caller passing 0 or a negative number would delete
  -- the current day and hand out fresh limits; one day is the least this can
  -- ever keep.
  IF keep_days IS NULL OR keep_days < 1 THEN
    keep_days := 1;
  END IF;

  -- The same Asia/Jakarta day the limit is counted against. Using the server's
  -- UTC date would drop a day seven hours early for everyone.
  cutoff := (NOW() AT TIME ZONE 'Asia/Jakarta')::date - keep_days;

  DELETE FROM daily_digital_spend WHERE spend_date < cutoff;
  GET DIAGNOSTICS removed = ROW_COUNT;
  RETURN removed;
END
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION purge_daily_digital_spend(INT) IS
  'Removes daily spend rows older than keep_days (Asia/Jakarta). SECURITY DEFINER so the application can expire rows without holding DELETE on the table.';

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    -- EXECUTE is revoked from PUBLIC first: a definer function granted to
    -- everyone by default would undo the point of not granting DELETE.
    REVOKE ALL ON FUNCTION purge_daily_digital_spend(INT) FROM PUBLIC;
    GRANT EXECUTE ON FUNCTION purge_daily_digital_spend(INT) TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS purge_daily_digital_spend(INT);
