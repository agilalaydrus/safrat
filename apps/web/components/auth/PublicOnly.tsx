"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { authClient } from "@/lib/auth-client";
import { resolveLandingPath } from "@/lib/post-login";
import { invalidateMyAccessCache } from "@/lib/access-cache";

export function PublicOnly({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { isPending } = authClient.useSession();
  const [checking, setChecking] = useState(true);
  // authClient.useSession()'s isPending flips true again after any auth
  // mutation completes (sign-up, sign-in, etc.) as it revalidates its
  // cache — without this guard, the loading branch below would swap back
  // in on that flip and unmount `children`, wiping any local state a form
  // just set (e.g. AuthForm's "check your email" screen right after
  // sign-up). Only the *first* settle should gate rendering.
  const hasCheckedOnce = useRef(false);

  useEffect(() => {
    if (isPending || hasCheckedOnce.current) return;

    let cancelled = false;
    async function redirectAuthenticatedUser() {
      try {
        // Bypass the client cache so a recently rotated Better Auth session is used.
        const session = await authClient.getSession({ fetchOptions: { cache: "no-store" } });
        if (!session.data?.user || cancelled) return;

        // Landing here right after Google's redirect (or any other
        // fresh-session case this guard covers) — a stale MyAccess from a
        // different account tested earlier in this tab must not decide
        // where this identity lands. See the matching note in AuthForm.
        invalidateMyAccessCache();
        const path = await resolveLandingPath();
        if (!cancelled) router.replace(path);
      } catch {
        // A failed refresh must not lock a genuine signed-out user out of the form.
      } finally {
        if (!cancelled) {
          hasCheckedOnce.current = true;
          setChecking(false);
        }
      }
    }

    void redirectAuthenticatedUser();
    return () => { cancelled = true; };
  }, [isPending, router]);

  if (!hasCheckedOnce.current && (isPending || checking)) {
    return <main style={loading}><p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>Checking your session...</p></main>;
  }

  return <>{children}</>;
}

const loading: React.CSSProperties = {
  minHeight: "100vh",
  display: "grid",
  placeItems: "center",
  background: "var(--color-cream-100)",
};
