"use client";

import { authClient } from "./auth-client";
import { getMyAccessCached } from "./access-cache";

/**
 * Single resolver for "where should this identity land after signing in" —
 * used by every entry point (email/password via AuthForm, Google's redirect
 * back via PublicOnly) so operator staff, group leaders, and pilgrims share
 * one login on one domain instead of each surface hardcoding its own
 * redirect.
 *
 * Priority is by org_role, not a flat "is this person an org member"
 * boolean — Better Auth's organization plugin already tiers membership
 * into owner/admin/member, and a "member"-role account that also leads a
 * group (every Group Leader/Muttawwif must be an org member so they're
 * assignable from the admin Groups page — see GroupService in the Go API)
 * is functioning as a leader, not an administrator. Sending them to
 * /dashboard first because of an incidental membership row is wrong RBAC,
 * not just a rough edge:
 *   1. owner/admin org role -> /dashboard (actual administrators)
 *   2. leads >=1 group -> /leader (their real job, as Muttawwif)
 *   3. member-role org member with an active linked agent record ->
 *      /agent (Tour Leader) — must come after the leader check: every
 *      Group Leader is also auto-created as an agent (see
 *      EnsureAgentForLeader), but their primary surface is /leader
 *   4. member-role org member with no group and no agent -> /dashboard
 *      (plain staff; it's the only surface they have anything to do on)
 *   5. linked pilgrim -> /pilgrim
 *   6. no role at all yet -> /onboarding ("create your operator")
 */
export async function resolveLandingPath(): Promise<string> {
  const access = await getMyAccessCached();

  if (access.isOrgMember) {
    // The org-activation dance predates this resolver — Better Auth session
    // rotation can drop activeOrganizationId while keeping the user session,
    // so re-set it whenever it's missing. Done regardless of where this
    // identity actually lands, since they may still visit /dashboard later.
    const session = await authClient.getSession({ fetchOptions: { cache: "no-store" } });
    if (!session.data?.session?.activeOrganizationId) {
      const organizations = await authClient.organization.list();
      const organization = organizations.data?.[0];
      if (organization) {
        await authClient.organization.setActive({ organizationId: organization.id });
      }
    }
  }

  const isAdmin = access.isOrgMember && (access.orgRole === "owner" || access.orgRole === "admin");
  if (isAdmin) return "/dashboard";
  if (access.leaderGroups.length > 0) return "/leader";
  if (access.isOrgMember && access.linkedAgent?.isActive) return "/agent";
  if (access.isOrgMember) return "/dashboard";
  if (access.linkedPilgrim) return "/pilgrim";
  return "/onboarding";
}
