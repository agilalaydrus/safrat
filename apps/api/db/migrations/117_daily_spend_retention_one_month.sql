-- +goose Up

-- Three days was too short (owner's decision). A month covers a full billing
-- cycle and any dispute that surfaces after a statement, which three days did
-- not: a jamaah querying a charge from last week had nothing left to reconcile
-- against.
--
-- The function's own default is moved with it. Leaving it at three while the
-- policy is thirty would be a trap for anyone calling this by hand during an
-- incident — the argument-less call would quietly delete four weeks of history.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION purge_daily_digital_spend(keep_days INT DEFAULT 30)
RETURNS INT
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  removed INT;
  cutoff DATE;
BEGIN
  IF keep_days IS NULL OR keep_days < 1 THEN
    keep_days := 1;
  END IF;
  cutoff := (NOW() AT TIME ZONE 'Asia/Jakarta')::date - keep_days;
  DELETE FROM daily_digital_spend WHERE spend_date < cutoff;
  GET DIAGNOSTICS removed = ROW_COUNT;
  RETURN removed;
END
$$;
-- +goose StatementEnd

-- CREATE OR REPLACE keeps the existing grants, but only because the signature
-- is unchanged. Say it rather than rely on remembering it: change the argument
-- list and this becomes a different function with default privileges, which
-- means EXECUTE granted to PUBLIC and the point of migration 115 quietly lost.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE ALL ON FUNCTION purge_daily_digital_spend(INT) FROM PUBLIC;
    GRANT EXECUTE ON FUNCTION purge_daily_digital_spend(INT) TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION purge_daily_digital_spend(keep_days INT DEFAULT 3)
RETURNS INT
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  removed INT;
  cutoff DATE;
BEGIN
  IF keep_days IS NULL OR keep_days < 1 THEN
    keep_days := 1;
  END IF;
  cutoff := (NOW() AT TIME ZONE 'Asia/Jakarta')::date - keep_days;
  DELETE FROM daily_digital_spend WHERE spend_date < cutoff;
  GET DIAGNOSTICS removed = ROW_COUNT;
  RETURN removed;
END
$$;
-- +goose StatementEnd
