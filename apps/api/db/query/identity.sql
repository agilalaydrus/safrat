-- name: GetOrgMembershipForUser :one
-- A Better Auth user can technically belong to more than one organization;
-- this app only ever treats one as "the" operator for a given identity, so
-- take any single membership row deterministically (oldest first) rather
-- than picking arbitrarily on each call.
SELECT m."organizationId" AS organization_id, m.role, o.id AS operator_id, o.name AS operator_name
FROM member m
JOIN operators o ON o.better_auth_org_id = m."organizationId"
WHERE m."userId" = $1
ORDER BY m."createdAt" ASC
LIMIT 1;

-- name: ListLeaderGroupsForUser :many
SELECT id, name FROM groups WHERE leader_id = $1 ORDER BY name ASC;

-- name: GetLinkedPilgrimForUser :one
SELECT id, app_access_code, full_name FROM pilgrims
WHERE linked_user_id = $1 AND is_substituted = false;
