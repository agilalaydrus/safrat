-- +goose Up

-- daily_digital_spend decides whether a purchase is allowed, so deleting a row
-- from it hands the account its whole daily limit back. The application never
-- deletes one — spending increments, reversal decrements — so the privilege
-- exists for no reason and only widens what a compromised application
-- credential can do.
--
-- Same reasoning as migration 100 applied to the ledgers, and the same limit
-- on it: this constrains the application role, not a superuser. It raises the
-- cost of a stolen DATABASE_URL, which is the realistic threat, rather than
-- claiming to stop someone who already owns the database.
--
-- UPDATE is deliberately kept. Unlike a ledger this is a counter, and a
-- refunded order has to be able to give its headroom back.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE DELETE, TRUNCATE ON daily_digital_spend FROM safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    GRANT DELETE ON daily_digital_spend TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd
