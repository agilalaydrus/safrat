// Builds a public link on the operator's own subdomain (e.g.
// vacana.tawafiqhub.id/register/[seasonId]) instead of the old
// operatorId-in-path form (app.tawafiqhub.id/register/[operatorId]/[seasonId])
// — apps/web/middleware.ts resolves the subdomain back to operatorId and
// rewrites onto the same underlying route, so the page itself is unchanged.
//
// 127.0.0.1 can't carry a subdomain, so local dev substitutes "localhost"
// (browsers resolve *.localhost to loopback automatically — no /etc/hosts
// entry needed). In production, a leading "app."/"www." on the current
// origin is stripped before prepending the slug.
export function buildTenantLink(slug: string, path: string): string {
  if (typeof window === "undefined" || !slug) return "";
  const { protocol, hostname, port } = window.location;
  const portSuffix = port ? `:${port}` : "";

  let base = hostname;
  if (hostname === "127.0.0.1" || hostname === "localhost") {
    base = "localhost";
  } else {
    const labels = hostname.split(".");
    if (labels.length > 2 && (labels[0] === "app" || labels[0] === "www")) {
      base = labels.slice(1).join(".");
    }
  }

  return `${protocol}//${slug}.${base}${portSuffix}${path}`;
}
