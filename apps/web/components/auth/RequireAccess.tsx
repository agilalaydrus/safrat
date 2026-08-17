"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { authClient } from "@/lib/auth-client";
import { getMyAccessCached } from "@/lib/access-cache";
import { resolveLandingPath } from "@/lib/post-login";

type Role = "staff" | "leader" | "pilgrim";

/**
 * Gate for /dashboard, /leader, and /pilgrim. middleware.ts only checks
 * that a Better Auth session cookie is present — it can't check *who* that
 * session belongs to (Edge middleware has no DB access), so without this,
 * a signed-in pilgrim or a leader-only identity could load the wrong
 * surface's shell and only find out they don't belong there when
 * individual RPC calls start 401ing underneath them. This does the real
 * check — the same UUID-backed MyAccess resolution every login redirect
 * already uses — and sends a wrong-role visitor to where they actually
 * belong instead of bouncing them to /sign-in (they ARE signed in, just
 * not for this surface).
 */
export function RequireAccess({ role, children }: { role: Role; children: React.ReactNode }) {
  const router = useRouter();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function check() {
      const session = await authClient.getSession({ fetchOptions: { cache: "no-store" } });
      if (cancelled) return;
      if (!session.data?.user) {
        router.replace("/sign-in");
        return;
      }
      try {
        const access = await getMyAccessCached();
        if (cancelled) return;
        const allowed = role === "staff" ? access.isOrgMember : role === "leader" ? access.leaderGroups.length > 0 : !!access.linkedPilgrim;
        if (allowed) {
          setChecking(false);
          return;
        }
        router.replace(await resolveLandingPath());
      } catch {
        if (!cancelled) router.replace("/sign-in");
      }
    }
    void check();
    return () => { cancelled = true; };
  }, [role, router]);

  if (checking) {
    return <main style={loading}><p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>Memeriksa akses Anda...</p></main>;
  }
  return <>{children}</>;
}

const loading: React.CSSProperties = {
  minHeight: "100vh",
  display: "grid",
  placeItems: "center",
  background: "var(--color-cream-100)",
};
