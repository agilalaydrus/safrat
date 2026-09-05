-- +goose Up
-- F3 (TUGAS-PANEL-SAAS.md): §9 DESAIN says plainly that dunning_log,
-- privileged_actions, and impersonation_sessions are evidence, not cache,
-- and the application role must not be able to delete any of them —
-- migrations 139 and 138 did this for the first two; impersonation_sessions
-- was never given the same protection. A leaked application credential
-- could otherwise erase the one durable record of who looked at a tenant's
-- dashboard as them, and when — the exact record migration 149's own
-- comment says makes impersonation "safe rather than a master key".
--
-- Column-scoped rather than a blanket UPDATE revoke, unlike audit_logs:
-- ImpersonationRepository.End legitimately updates ended_at/ended_reason
-- when a session closes. What must never move is the evidence of the
-- session itself — who, which tenant, why, when it started, and the key
-- that made opening it idempotent.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE UPDATE (admin_user_id, operator_id, token_hash, reason, ip, user_agent, started_at, expires_at, idempotency_key)
      ON impersonation_sessions FROM safrat_app;
    REVOKE DELETE, TRUNCATE ON impersonation_sessions FROM safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    GRANT UPDATE ON impersonation_sessions TO safrat_app;
    GRANT DELETE ON impersonation_sessions TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd
