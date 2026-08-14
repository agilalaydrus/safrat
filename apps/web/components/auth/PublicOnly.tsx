"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { authClient } from "@/lib/auth-client";

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

        const organizations = await authClient.organization.list();
        if (cancelled) return;

        const firstOrganization = organizations.data?.[0];
        if (!firstOrganization) {
          router.replace("/onboarding");
          router.refresh();
          return;
        }

        // Session rotation can drop the active organization while preserving the user session.
        if (!session.data.session?.activeOrganizationId) {
          await authClient.organization.setActive({ organizationId: firstOrganization.id });
          await authClient.getSession({ fetchOptions: { cache: "no-store" } });
        }

        if (!cancelled) {
          router.replace("/dashboard");
          router.refresh();
        }
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
