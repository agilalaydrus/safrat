// Keep this list aligned with operatorReservedSlugs in the API and the
// operators_slug_policy_check database constraint. The API/database remain
// authoritative; the web list prevents platform hostnames from being treated
// as tenant storefronts before a dedicated service is attached to them.
export const RESERVED_TENANT_SUBDOMAINS = new Set([
  "admin",
  "api",
  "app",
  "auth",
  "dashboard",
  "docs",
  "help",
  "status",
  "support",
  "www",
]);

export const PLATFORM_BASE_HOSTS = ["tawafiqhub.id", "safrat.com", "localhost"];

const DNS_LABEL_PATTERN = /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/;

export function isUsableTenantSlug(slug: string): boolean {
  return (
    slug.length >= 3 &&
    slug.length <= 63 &&
    DNS_LABEL_PATTERN.test(slug) &&
    !RESERVED_TENANT_SUBDOMAINS.has(slug)
  );
}

function normalizedHostname(host: string): string {
  // Tenant hosts are DNS names, so a colon can only be the local development
  // port separator here (e.g. vacana.localhost:3131).
  return (host.split(":")[0] ?? "").toLowerCase().replace(/\.$/, "");
}

export function platformBaseHostname(rawHostname: string): string {
  const hostname = normalizedHostname(rawHostname);
  if (hostname === "127.0.0.1" || hostname === "localhost") return "localhost";
  for (const base of PLATFORM_BASE_HOSTS) {
    if (hostname === base || hostname.endsWith(`.${base}`)) return base;
  }
  return hostname;
}

export function extractTenantSlug(host: string): string | null {
  const hostname = normalizedHostname(host);
  for (const base of PLATFORM_BASE_HOSTS) {
    if (hostname === base) return null;
    if (!hostname.endsWith(`.${base}`)) continue;

    const subdomain = hostname.slice(0, -(base.length + 1));
    if (subdomain.includes(".") || !isUsableTenantSlug(subdomain)) {
      return null;
    }
    return subdomain;
  }
  return null;
}
