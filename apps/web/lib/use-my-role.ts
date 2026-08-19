"use client";

import { authClient } from "@/lib/auth-client";

export type OrgRole = "owner" | "admin" | "member";

/**
 * Better Auth's organization plugin exposes the signed-in user's role on
 * the *active* organization via the `activeMemberRole` atom, which the
 * react client auto-generates as `useActiveMemberRole()` (atom key ->
 * `use${capitalize(key)}`) — a real DB-backed lookup, not something read
 * off the session cookie.
 */
export function useMyRole(): { role: OrgRole | null; isOwner: boolean; isAdmin: boolean; isPending: boolean } {
  const { data, isPending } = authClient.useActiveMemberRole();
  const role = (data?.role as OrgRole | undefined) ?? null;
  return {
    role,
    isOwner: role === "owner",
    isAdmin: role === "owner" || role === "admin",
    isPending,
  };
}
