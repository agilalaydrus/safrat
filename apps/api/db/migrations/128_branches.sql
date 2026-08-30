-- +goose Up
-- Branch offices under a head office.
--
-- Until now an operator was flat: every staff member saw every jamaah. A
-- travel agency with offices in five cities needs its Bandung branch head to
-- work their own portfolio and nobody else's — and under UU PDP, seeing
-- another branch's jamaah is not an inconvenience, it is unauthorised access
-- to personal data.
--
-- branch_id is nullable everywhere on purpose. NULL means "held by the head
-- office", which is what every existing row is, so this migration changes no
-- behaviour for operators who never create a branch.

CREATE TABLE branches (
  id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id        UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  name               TEXT        NOT NULL CHECK (length(trim(name)) > 0),
  city               TEXT        NOT NULL DEFAULT '',
  -- Targets are what the branch is measured against. Zero means "not set",
  -- which reads the same as no target and avoids a nullable number that
  -- every report would have to special-case.
  target_pilgrims    INTEGER     NOT NULL DEFAULT 0 CHECK (target_pilgrims >= 0),
  target_revenue_idr BIGINT      NOT NULL DEFAULT 0 CHECK (target_revenue_idr >= 0),
  phone              TEXT        NOT NULL DEFAULT '',
  -- A branch collects money into its own account before settling with head
  -- office, so the account belongs to the branch, not to the operator.
  bank_name          TEXT        NOT NULL DEFAULT '',
  account_number     TEXT        NOT NULL DEFAULT '',
  account_holder     TEXT        NOT NULL DEFAULT '',
  is_active          BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Two branches with the same name in one operator makes every report
-- ambiguous and every dropdown a guess.
CREATE UNIQUE INDEX branches_operator_name_key ON branches (operator_id, lower(trim(name)));
CREATE INDEX branches_operator_idx ON branches (operator_id) WHERE is_active;

-- Who runs which branch. Better Auth owns users and org roles, so this maps
-- its user id to a branch rather than adding a column to a table we do not
-- own.
--
-- The primary key is the user id, so "a branch head cannot hold two branches"
-- is enforced by the database. A check-then-insert would let two concurrent
-- assignments both pass.
CREATE TABLE branch_members (
  better_auth_user_id TEXT        PRIMARY KEY,
  branch_id           UUID        NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  operator_id         UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX branch_members_branch_idx ON branch_members (branch_id);
CREATE INDEX branch_members_operator_idx ON branch_members (operator_id);

-- RESTRICT, not CASCADE and not SET NULL. Deleting a branch must not delete
-- the jamaah registered through it, and must not silently move them to head
-- office either — that would quietly rewrite who is responsible for them.
-- Move them out first, deliberately.
ALTER TABLE pilgrims              ADD COLUMN branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT;
ALTER TABLE pilgrim_registrations ADD COLUMN branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT;
ALTER TABLE agents                ADD COLUMN branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT;
ALTER TABLE orders                ADD COLUMN branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT;

-- Partial: the overwhelming majority of rows are NULL (head office) and
-- indexing those helps nothing.
CREATE INDEX pilgrims_branch_idx              ON pilgrims (branch_id)              WHERE branch_id IS NOT NULL;
CREATE INDEX pilgrim_registrations_branch_idx ON pilgrim_registrations (branch_id) WHERE branch_id IS NOT NULL;
CREATE INDEX agents_branch_idx                ON agents (branch_id)                WHERE branch_id IS NOT NULL;
CREATE INDEX orders_branch_idx                ON orders (branch_id)                WHERE branch_id IS NOT NULL;

-- A branch must belong to the same operator as the row pointing at it.
-- Without this, a stray branch id from another tenant would scope a query to
-- nothing — or worse, to somebody else's rows if the operator filter were
-- ever dropped. Enforced as a trigger because a foreign key cannot span the
-- operator column.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_branch_matches_operator() RETURNS trigger AS $$
BEGIN
  IF NEW.branch_id IS NULL THEN
    RETURN NEW;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM branches b
    WHERE b.id = NEW.branch_id AND b.operator_id = NEW.operator_id
  ) THEN
    RAISE EXCEPTION 'branch % does not belong to operator %', NEW.branch_id, NEW.operator_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER pilgrims_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON pilgrims
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();
CREATE TRIGGER pilgrim_registrations_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON pilgrim_registrations
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();
CREATE TRIGGER agents_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON agents
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();
CREATE TRIGGER orders_branch_matches_operator
  BEFORE INSERT OR UPDATE OF branch_id, operator_id ON orders
  FOR EACH ROW EXECUTE FUNCTION assert_branch_matches_operator();

-- +goose Down
DROP TRIGGER IF EXISTS orders_branch_matches_operator ON orders;
DROP TRIGGER IF EXISTS agents_branch_matches_operator ON agents;
DROP TRIGGER IF EXISTS pilgrim_registrations_branch_matches_operator ON pilgrim_registrations;
DROP TRIGGER IF EXISTS pilgrims_branch_matches_operator ON pilgrims;
DROP FUNCTION IF EXISTS assert_branch_matches_operator();
DROP INDEX IF EXISTS orders_branch_idx;
DROP INDEX IF EXISTS agents_branch_idx;
DROP INDEX IF EXISTS pilgrim_registrations_branch_idx;
DROP INDEX IF EXISTS pilgrims_branch_idx;
ALTER TABLE orders                DROP COLUMN IF EXISTS branch_id;
ALTER TABLE agents                DROP COLUMN IF EXISTS branch_id;
ALTER TABLE pilgrim_registrations DROP COLUMN IF EXISTS branch_id;
ALTER TABLE pilgrims              DROP COLUMN IF EXISTS branch_id;
DROP TABLE IF EXISTS branch_members;
DROP TABLE IF EXISTS branches;
