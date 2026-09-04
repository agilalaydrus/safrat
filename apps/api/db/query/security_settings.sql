-- name: GetOperatorSecuritySettings :one
SELECT * FROM operator_security_settings WHERE operator_id = $1;

-- name: UpsertOperatorSecuritySettings :one
INSERT INTO operator_security_settings (operator_id, ip_allowlist_enabled, ip_allowlist_cidrs, updated_by_user_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (operator_id) DO UPDATE
  SET ip_allowlist_enabled = EXCLUDED.ip_allowlist_enabled,
      ip_allowlist_cidrs = EXCLUDED.ip_allowlist_cidrs,
      updated_by_user_id = EXCLUDED.updated_by_user_id
RETURNING *;

-- name: ListOperatorActiveSessions :many
-- Every live session belonging to a member of this operator's organization —
-- raw SQL against Better Auth's own tables, the same pattern
-- internal/middleware/auth.go already uses to resolve a session.
SELECT s.id, s."userId", u.name, u.email, s."ipAddress", s."userAgent", s."createdAt", s."expiresAt"
FROM session s
JOIN "user" u ON u.id = s."userId"
JOIN member m ON m."userId" = s."userId"
JOIN operators o ON o.better_auth_org_id = m."organizationId"
WHERE o.id = $1 AND s."expiresAt" > NOW()
ORDER BY s."createdAt" DESC;

-- name: RevokeOperatorSession :execrows
-- Scoped through the same operator/member join as the list above, so a
-- session id from another tenant can never be named and deleted from here.
DELETE FROM session s
USING member m, operators o
WHERE s.id = $1
  AND m."userId" = s."userId"
  AND o.better_auth_org_id = m."organizationId"
  AND o.id = $2;
