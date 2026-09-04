-- +goose Up
-- Kebijakan Keamanan (§4.9 DESAIN): most of what that screen wants is
-- already true and just not shown anywhere — 2FA is mandatory for every
-- staff session (RequireTwoFactor "enforce" in the web app) and signing in
-- anywhere already revokes every other session for that user
-- (apps/web/lib/auth.ts's session.create hook), both unconditional, both
-- stricter than "configurable per operator". The one real gap is IP
-- restriction, which does not exist in any form yet — this table is that.
--
-- Enabled defaults to false: an operator who never visits this screen must
-- see no behaviour change. Turning it on is a real way to lock out every
-- device that isn't on the list, including the person turning it on — the
-- service layer refuses to enable it unless the caller's own current
-- request IP already matches one of the CIDRs being saved.
CREATE TABLE operator_security_settings (
  operator_id          UUID        PRIMARY KEY REFERENCES operators(id) ON DELETE CASCADE,
  ip_allowlist_enabled BOOLEAN     NOT NULL DEFAULT FALSE,
  ip_allowlist_cidrs   TEXT[]      NOT NULL DEFAULT '{}',
  -- Enabled with an empty list would block every request including the
  -- owner's — the one configuration that can never be reached from any IP.
  CHECK (NOT ip_allowlist_enabled OR array_length(ip_allowlist_cidrs, 1) > 0),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by_user_id   TEXT        NOT NULL DEFAULT ''
);
CREATE TRIGGER operator_security_settings_set_updated_at BEFORE UPDATE ON operator_security_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS operator_security_settings;
