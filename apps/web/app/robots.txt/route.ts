import { tenantFromHost } from "@/lib/tenant-from-host";

export const dynamic = "force-dynamic";

/**
 * robots.txt, answered per hostname.
 *
 * One file cannot serve both the platform and every client domain: the
 * platform has an admin area to keep out of indexes, a storefront has none,
 * and a hostname nobody has been confirmed to own must not be indexed at all.
 */
export async function GET(request: Request) {
  const host = request.headers.get("host") ?? "";
  const tenant = await tenantFromHost(host);
  const sitemap = `https://${host}/sitemap.xml`;

  if (tenant.kind === "unknown") {
    // Reserved subdomains, unverified domains, anything we cannot attribute.
    return text("User-agent: *\nDisallow: /\n");
  }

  if (tenant.kind === "platform") {
    return text(
      [
        "User-agent: *",
        "Allow: /",
        // Everything behind a login. None of it is reachable without a session,
        // but a crawler spending its budget on redirects is a crawler spending
        // less of it on pages that matter.
        "Disallow: /dashboard",
        "Disallow: /admin",
        "Disallow: /leader",
        "Disallow: /pilgrim",
        "Disallow: /agent",
        "Disallow: /onboarding",
        "Disallow: /sign-in",
        "Disallow: /sign-up",
        "Disallow: /reset-password",
        "Disallow: /forgot-password",
        "Disallow: /two-factor-challenge",
        "Disallow: /accept-invitation",
        "Disallow: /storefront-preview",
        "Disallow: /api",
        "",
        `Sitemap: ${sitemap}`,
        "",
      ].join("\n"),
    );
  }

  return text(
    [
      "User-agent: *",
      "Allow: /",
      // A pilgrim's tracking page and certificate are addressable by code. They
      // are not secret, but they are personal, and they have no business in a
      // search index.
      "Disallow: /track",
      "Disallow: /certificate",
      "Disallow: /pilgrim",
      "Disallow: /api",
      "",
      `Sitemap: ${sitemap}`,
      "",
    ].join("\n"),
  );
}

function text(body: string): Response {
  return new Response(body, {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "public, max-age=300, s-maxage=3600",
    },
  });
}
