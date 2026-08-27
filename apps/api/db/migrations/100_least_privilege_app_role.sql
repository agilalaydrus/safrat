-- +goose Up

-- The application connects as the database superuser today, which makes the
-- append-only guarantee weaker than it looks: a superuser can disable the
-- triggers with one statement, or drop the tables outright. "Ledger entries
-- cannot be edited" is only true of code that plays along.
--
-- Verified rather than assumed, against this schema's actual shape:
--
--   direct UPDATE on a ledger table   → refused by privilege
--   direct DELETE on a ledger table   → refused by privilege
--   ALTER TABLE ... DISABLE TRIGGER   → refused: not the owner
--   DROP TABLE                        → refused: not the owner
--   ON DELETE CASCADE from a parent   → still works
--
-- That last one matters both ways. Cascades keep working, so tenant teardown
-- is unaffected — but it also means privileges alone would let ledger rows be
-- removed indirectly by deleting an operator. The append-only trigger is what
-- stops that, and the app role cannot disable it. The two controls cover each
-- other's gap; neither is sufficient alone.
--
-- Roles are cluster-wide rather than per-database, so this is written to be
-- safe to re-run and safe on a database where the role already exists.

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    -- NOLOGIN here: the password is set out of band during cutover, so it
    -- never appears in a migration file or in git.
    CREATE ROLE safrat_app NOLOGIN;
  END IF;
END $$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO safrat_app;

-- Everything the application legitimately writes.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO safrat_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO safrat_app;

-- ...and then the money records are narrowed to append-only, which is the
-- whole point. Reading and adding, nothing else.
REVOKE UPDATE, DELETE, TRUNCATE ON
  agent_commission_entries,
  pilgrim_balance_entries,
  order_refunds,
  supplier_cost_observations
FROM safrat_app;

-- Tables created by later migrations would otherwise arrive with no grants at
-- all, and the application would start failing on them the moment they are
-- used. Anything money-related added later must have its UPDATE/DELETE revoked
-- explicitly in that migration — the default is deliberately permissive so a
-- forgotten grant cannot take the app down, and a forgotten revoke is caught
-- by the audit query in DEPLOY.md instead.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO safrat_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO safrat_app;

-- Better Auth manages its own tables and runs its own migrations as the owner;
-- the application still needs to read and write sessions through them.
-- Nothing here changes that.

-- +goose Down
-- Deliberately does not drop the role: it may own nothing, but revoking a live
-- application's access as part of a rollback would turn a schema rollback into
-- an outage. Drop it by hand if it is genuinely unwanted.
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM safrat_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM safrat_app;
REVOKE USAGE ON SCHEMA public FROM safrat_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM safrat_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  REVOKE USAGE, SELECT ON SEQUENCES FROM safrat_app;
