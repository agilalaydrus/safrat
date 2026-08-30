-- +goose Up

-- The audit log is the evidence trail, and the application role could rewrite
-- it. Nothing in the application updates or deletes an audit row — the only
-- DELETE anywhere is inside migration 108, which runs as the owner — so the
-- privilege bought nothing and cost the one property the log exists for.
--
-- This matters more since docs/INSIDEN-DATA-PRIBADI.md: the 72-hour question
-- "whose data was read" is answered entirely from this table. A stolen
-- DATABASE_URL that can erase its own reads makes that answer unavailable
-- exactly when it is needed, and does so silently.
--
-- Same limit as migration 100 and 115, stated rather than implied: this
-- constrains the application role, not a superuser. It raises the cost of a
-- leaked application credential, which is the realistic threat.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- KYC records keep UPDATE: key rotation rewrites the ciphertext in place, and
-- verification moves the status. DELETE goes — nothing in the application
-- deletes one, and a data subject exercising erasure under UU PDP is a
-- deliberate act by an operator with owner access, not something an
-- application credential should be able to do on its own.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE DELETE, TRUNCATE ON kyc_records FROM safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    GRANT UPDATE, DELETE ON audit_logs TO safrat_app;
    GRANT DELETE ON kyc_records TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd
