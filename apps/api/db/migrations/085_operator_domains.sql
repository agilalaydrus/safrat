-- +goose Up
-- Tenancy has so far been *derived* from the hostname: strip the subdomain,
-- look up the slug. That only works while every tenant lives under a platform
-- domain we own. A client bringing their own domain has no slug in the
-- hostname, so identity has to be stored rather than inferred.
--
-- Deliberately additive: platform subdomains keep resolving exactly as before
-- (see extractTenantSlug in apps/web/lib/tenant-host.ts). This table holds the
-- domains that cannot be derived — which today means custom client domains.
-- Nothing is backfilled, so existing tenants are untouched.
--
-- This one table is intended to drive routing, CORS/trusted origins, and
-- on-demand TLS issuance, so a hostname is trusted in exactly one place.
CREATE TABLE operator_domains (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  -- Stored lowercase and without port or trailing dot; the resolver normalizes
  -- before lookup so a Host header can never miss on case alone.
  hostname      TEXT NOT NULL UNIQUE
                CHECK (hostname = lower(hostname)
                       AND hostname !~ '[:/ ]'
                       AND hostname NOT LIKE '%.'
                       AND length(hostname) BETWEEN 4 AND 253),
  -- Ownership is proven before the domain is served: the operator publishes
  -- this token as a DNS TXT record. Unverified rows must never be routed to,
  -- must never be added to a CORS allowlist, and must never be issued a
  -- certificate — otherwise anyone could point a hostname at us and claim it.
  verification_token TEXT NOT NULL DEFAULT encode(gen_random_bytes(16), 'hex'),
  verified_at   TIMESTAMPTZ,
  -- The canonical hostname for this operator, used when we need to generate an
  -- absolute URL (emails, OAuth handoff targets) rather than echo a request.
  is_primary    BOOLEAN NOT NULL DEFAULT false,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Routing looks up verified hostnames on nearly every request.
CREATE INDEX operator_domains_verified_idx ON operator_domains (hostname) WHERE verified_at IS NOT NULL;
CREATE INDEX operator_domains_operator_idx ON operator_domains (operator_id);
-- At most one primary per operator, and only a verified domain may be primary.
CREATE UNIQUE INDEX operator_domains_primary_idx ON operator_domains (operator_id) WHERE is_primary;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION operator_domains_guard() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := NOW();
  IF NEW.is_primary AND NEW.verified_at IS NULL THEN
    RAISE EXCEPTION 'an unverified domain cannot be primary';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER operator_domains_guard_trigger
  BEFORE INSERT OR UPDATE ON operator_domains
  FOR EACH ROW EXECUTE FUNCTION operator_domains_guard();

-- +goose Down
DROP TRIGGER IF EXISTS operator_domains_guard_trigger ON operator_domains;
DROP FUNCTION IF EXISTS operator_domains_guard();
DROP TABLE IF EXISTS operator_domains;
