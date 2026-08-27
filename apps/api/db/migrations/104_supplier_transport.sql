-- +goose Up

-- How to *reach* a supplier, alongside how to read what they say back.
--
-- The shapes in this market do not converge: some are REST with a JSON body,
-- plenty are a plain GET with everything in the query string, and the older
-- host-to-host terminals are XML-RPC. Writing a client per supplier means a
-- deploy every time one is added, which is the same trap the response parsing
-- avoided — so the request is configuration too.
ALTER TABLE suppliers
  ADD COLUMN protocol TEXT NOT NULL DEFAULT 'REST_JSON'
    CHECK (protocol IN ('REST_JSON', 'HTTP_GET', 'FORM_POST', 'XML_RPC')),
  ADD COLUMN http_method TEXT NOT NULL DEFAULT 'POST'
    CHECK (http_method IN ('GET', 'POST')),
  -- Bounded per supplier: a terminal that hangs must not hold a worker slot
  -- while other jamaah wait behind it.
  ADD COLUMN timeout_seconds INT NOT NULL DEFAULT 20
    CHECK (timeout_seconds BETWEEN 1 AND 120),
  -- Host-to-host terminals almost universally want a hash of some concatenated
  -- fields — "md5:{{username}}{{api_key}}{{reference}}" and the like. Stored as
  -- a recipe rather than code for the same reason as everything else here.
  --
  -- md5 is weak, and is nonetheless what most of these providers mandate. It is
  -- supported because refusing it would mean refusing the suppliers, not
  -- because it is a good choice.
  ADD COLUMN signature_recipe TEXT NOT NULL DEFAULT '',
  -- XML-RPC needs a method name; the others ignore it.
  ADD COLUMN rpc_method TEXT NOT NULL DEFAULT '';

-- The credential env var already existed but had no companion for a second
-- secret, which these providers usually need (a username *and* an api key).
ALTER TABLE suppliers
  ADD COLUMN username_env_var TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN suppliers.request_template IS
  'Request body or query string, with {{sku}}, {{reference}}, {{amount}}, {{destination}}, {{username}}, {{credential}}, {{signature}}, {{timestamp}} substituted at send time. Never store a credential here — name its environment variable instead.';

-- +goose Down
ALTER TABLE suppliers
  DROP COLUMN username_env_var,
  DROP COLUMN rpc_method,
  DROP COLUMN signature_recipe,
  DROP COLUMN timeout_seconds,
  DROP COLUMN http_method,
  DROP COLUMN protocol;
