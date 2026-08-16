"use client";

import { authClient } from "./auth-client";
import { identityClient } from "./rpc";

/**
 * Single resolver for "where should this identity land after signing in" —
 * used by every entry point (email/password via AuthForm, Google's redirect
 * back via PublicOnly) so operator staff, group leaders, and pilgrims share
 * one login on one domain instead of each surface hardcoding its own
 * redirect. Priority when an identity has more than one role: operator
 * staff > group leader > linked pilgrim > no role yet (onboarding, i.e.
 * "create your operator").
 */
export async function resolveLandingPath(): Promise<string> {
  const access = await identityClient.getMyAccess({});

  if (access.isOrgMember) {
    // The org-activation dance predates this resolver — Better Auth session
    // rotation can drop activeOrganizationId while keeping the user session,
    // so re-set it whenever it's missing.
    const session = await authClient.getSession({ fetchOptions: { cache: "no-store" } });
    if (!session.data?.session?.activeOrganizationId) {
      const organizations = await authClient.organization.list();
      const organization = organizations.data?.[0];
      if (organization) {
        await authClient.organization.setActive({ organizationId: organization.id });
      }
    }
    return "/dashboard";
  }

  if (access.leaderGroups.length > 0) return "/leader";
  if (access.linkedPilgrim) return `/pilgrim/${access.linkedPilgrim.appAccessCode}`;
  return "/onboarding";
}
