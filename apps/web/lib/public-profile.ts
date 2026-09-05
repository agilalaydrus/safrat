import type { StorefrontProfile } from "@/components/storefront/TenantStorefront";

// Server-side fetch of a tenant's public profile (storefront pages, article
// pages, sitemap.xml — every server-rendered surface that needs one).
//
// Deliberately bypasses lib/rpc.ts's Connect client: that client's transport
// resolves the Better Auth session via a browser-only singleton, which does
// not exist in a Server Component or Route Handler. GetPublicProfile is a
// public, unauthenticated RPC (see publicProcedures in
// apps/api/internal/middleware/auth.go) — nothing here needs a session
// anyway. Every server-side caller must go through this one function: it
// used to be copy-pasted four times, each with its own hand-typed subset of
// the response shape, silently free to drift from what the API actually
// returns.
export async function getPublicProfile(slug: string): Promise<StorefrontProfile | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/GetPublicProfile`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ slug }),
      cache: "no-store",
    });
    if (!response.ok) return null;
    return (await response.json()) as StorefrontProfile;
  } catch {
    return null;
  }
}
