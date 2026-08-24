// Builds a public link on the operator's own subdomain (e.g.
// vacana.tawafiqhub.id/register/[seasonId]) instead of the old
// operatorId-in-path form (app.tawafiqhub.id/register/[operatorId]/[seasonId])
// — apps/web/middleware.ts resolves the subdomain back to operatorId and
// rewrites onto the same underlying route, so the page itself is unchanged.
//
const PLATFORM_BASE_HOSTS = ["tawafiqhub.id", "safrat.com", "localhost"];

function platformBaseHostname(hostname: string): string {
  if (hostname === "127.0.0.1" || hostname === "localhost") return "localhost";
  for (const base of PLATFORM_BASE_HOSTS) {
    if (hostname === base || hostname.endsWith(`.${base}`)) return base;
  }
  return hostname;
}

// 127.0.0.1 can't carry a subdomain, so local dev substitutes "localhost"
// (browsers resolve *.localhost to loopback automatically — no /etc/hosts
// entry needed). Existing app/www/operator prefixes are reduced to the known
// platform base before the selected tenant slug is prepended.
export function buildTenantLinkFromBase(slug: string, path: string, baseUrl: string): string {
  if (!slug || !baseUrl) return "";
  const url = new URL(baseUrl);
  url.hostname = `${slug}.${platformBaseHostname(url.hostname)}`;
  url.pathname = path;
  url.search = "";
  url.hash = "";
  return url.toString();
}

export function buildTenantLink(slug: string, path: string): string {
  if (typeof window === "undefined" || !slug) return "";
  return buildTenantLinkFromBase(slug, path, window.location.href);
}
