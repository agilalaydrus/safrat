import { PLATFORM_BASE_HOSTS, RESERVED_TENANT_SUBDOMAINS, extractTenantSlug, isUsableTenantSlug } from "@/lib/tenant-host";

/**
 * Which storefront, if any, a hostname belongs to.
 *
 * Search engines ask for /robots.txt and /sitemap.xml on whatever hostname they
 * are crawling, and those files must describe that tenant and nobody else. A
 * sitemap that leaked another agency's URLs would hand a competitor the
 * customer list.
 */
export type HostTenant =
  | { kind: "platform" }
  | { kind: "tenant"; slug: string }
  /** A hostname we serve but cannot attribute — reserved, unverified, or
   *  unknown. Nothing here may be indexed: pages nobody has been confirmed to
   *  own should not enter a search index under anyone's name. */
  | { kind: "unknown" };

function normalise(host: string): string {
  return (host.split(":")[0] ?? "").toLowerCase().replace(/\.$/, "");
}

export function isPlatformHostname(host: string): boolean {
  const hostname = normalise(host);
  return PLATFORM_BASE_HOSTS.some((base) => hostname === base || hostname.endsWith(`.${base}`));
}

/**
 * Resolves a hostname to its tenant. Subdomains are read directly; a client's
 * own domain needs the API, which also tells us whether it is verified.
 */
export async function tenantFromHost(host: string): Promise<HostTenant> {
  const hostname = normalise(host);
  if (!hostname) return { kind: "unknown" };

  if (isPlatformHostname(hostname)) {
    const slug = extractTenantSlug(hostname);
    if (slug) return isUsableTenantSlug(slug) ? { kind: "tenant", slug } : { kind: "unknown" };

    // extractTenantSlug returns null for the apex and for every reserved
    // subdomain alike, and those are not the same thing. The apex and www are
    // the platform's own site; admin, api, app and the rest are internal, and
    // letting them answer as the platform would put the same pages in an index
    // under several hostnames.
    const subdomain = platformSubdomain(hostname);
    if (subdomain === "" || subdomain === "www") return { kind: "platform" };
    if (RESERVED_TENANT_SUBDOMAINS.has(subdomain)) return { kind: "unknown" };
    return { kind: "unknown" };
  }

  // A bare hostname with no dot is never a routable client domain, and this
  // keeps localhost and internal health checks from making a lookup.
  if (!hostname.includes(".")) return { kind: "unknown" };

  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/ResolveOperatorDomain`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ hostname }),
      cache: "no-store",
    });
    if (!response.ok) return { kind: "unknown" };
    const data = (await response.json()) as { slug?: string };
    return data.slug ? { kind: "tenant", slug: data.slug } : { kind: "unknown" };
  } catch {
    // An unreachable API must not turn into a permissive robots.txt. Refusing
    // to be indexed is recoverable; being wrongly indexed is not.
    return { kind: "unknown" };
  }
}

/** The label in front of a platform base host, or "" for the apex. */
function platformSubdomain(hostname: string): string {
  for (const base of PLATFORM_BASE_HOSTS) {
    if (hostname === base) return "";
    if (hostname.endsWith(`.${base}`)) return hostname.slice(0, -(base.length + 1));
  }
  return "";
}
