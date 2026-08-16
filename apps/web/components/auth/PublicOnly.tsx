"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { authClient } from "@/lib/auth-client";
import { resolveLandingPath } from "@/lib/post-login";

export function PublicOnly({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { isPending } = authClient.useSession();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    if (isPending) return;

    let cancelled = false;
    async function redirectAuthenticatedUser() {
      try {
        // Bypass the client cache so a recently rotated Better Auth session is used.
        const session = await authClient.getSession({ fetchOptions: { cache: "no-store" } });
        if (!session.data?.user || cancelled) return;

        const path = await resolveLandingPath();
        if (!cancelled) router.replace(path);
      } catch {
        // A failed refresh must not lock a genuine signed-out user out of the form.
      } finally {
        if (!cancelled) setChecking(false);
      }
    }

    void redirectAuthenticatedUser();
    return () => { cancelled = true; };
  }, [isPending, router]);

  if (isPending || checking) {
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
