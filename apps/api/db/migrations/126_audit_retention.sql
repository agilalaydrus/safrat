-- +goose Up

-- Audit rows are kept for two years, then removed.
--
-- The number is a judgement, so here is the reasoning rather than just the
-- constant. Breach notification runs on 72 hours, but an investigation looks
-- back much further — a credential quietly misused for months is exactly the
-- case this table exists to reconstruct. Two years covers any realistic
-- inquiry, including one opened long after the fact.
--
-- It is not ten years. This table is not the financial record: orders,
-- refunds and the ledgers are, and none of them are touched here. What this
-- holds is who looked at whose personal data — which is itself sensitive, and
-- keeping it forever means an ever-growing store of exactly the information a
-- breach would most want.
--
-- One constant, easy to change if a regulator asks for something else.
--
-- SECURITY DEFINER because migration 125 took DELETE on this table away from
-- the application role: the audit trail must not be erasable by the credential
-- being audited. The purge is bounded from inside, so the worker can expire old
-- rows and can reach nothing else.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION purge_audit_logs(keep_months INT DEFAULT 24)
RETURNS INT
LANGUAGE plpgsql
SECURITY DEFINER
-- Pinned, because a definer function without one is a way to run code as its
-- owner.
SET search_path = pg_catalog, public
AS $$
DECLARE
  removed INT;
BEGIN
  -- Clamped hard. A caller passing 0 would erase the whole trail, and this
  -- function runs with owner rights — the floor is what stops a mistake in a
  -- worker from destroying the evidence it is meant to preserve.
  IF keep_months IS NULL OR keep_months < 6 THEN
    keep_months := 6;
  END IF;

  DELETE FROM audit_logs
  WHERE created_at < NOW() - make_interval(months => keep_months);
  GET DIAGNOSTICS removed = ROW_COUNT;
  RETURN removed;
END
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION purge_audit_logs(INT) IS
  'Removes audit rows older than keep_months (floor 6). SECURITY DEFINER so the application can expire them without holding DELETE on the table it is audited by.';

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE ALL ON FUNCTION purge_audit_logs(INT) FROM PUBLIC;
    GRANT EXECUTE ON FUNCTION purge_audit_logs(INT) TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- Retention scans by age, and nothing indexed that.
CREATE INDEX IF NOT EXISTS audit_logs_created_at_idx ON audit_logs (created_at);

-- +goose Down
DROP INDEX IF EXISTS audit_logs_created_at_idx;
DROP FUNCTION IF EXISTS purge_audit_logs(INT);
