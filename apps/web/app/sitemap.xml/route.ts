import { tenantFromHost } from "@/lib/tenant-from-host";
import { getPublicProfile } from "@/lib/public-profile";

export const dynamic = "force-dynamic";

/**
 * sitemap.xml, built from the hostname being crawled.
 *
 * Only that tenant's URLs appear. A sitemap that leaked another agency's pages
 * would hand a competitor the customer list, so the tenant is resolved from
 * the host rather than passed in.
 */
export async function GET(request: Request) {
  const host = request.headers.get("host") ?? "";
  const tenant = await tenantFromHost(host);
  if (tenant.kind === "unknown") {
    return new Response("Not found", { status: 404 });
  }

  const base = `https://${host}`;
  if (tenant.kind === "platform") {
    return xml([{ loc: `${base}/` }, { loc: `${base}/keamanan` }]);
  }

  const profile = await getPublicProfile(tenant.slug);
  if (!profile) return new Response("Not found", { status: 404 });

  const entries: SitemapEntry[] = [{ loc: `${base}/` }];
  for (const season of profile.activeSeasons ?? []) {
    const slug = season.slug || season.id;
    if (slug) entries.push({ loc: `${base}/register/${profile.slug}/${slug}` });
  }
  // News and blog live on different paths but are the same kind of thing.
  for (const [path, articles] of [
    ["berita", profile.content?.news],
    ["blog", profile.content?.blogPosts],
  ] as const) {
    for (const article of articles ?? []) {
      if (!article.slug) continue;
      // publishedAt comes back from raw JSON here (not the protobuf-parsed
      // client), so it is always a plain RFC3339 string at runtime even
      // though the shared StorefrontArticle type also allows a Timestamp
      // object for callers that go through the generated proto client.
      entries.push({ loc: `${base}/${path}/${article.slug}`, lastmod: typeof article.publishedAt === "string" ? article.publishedAt : undefined });
    }
  }
  return xml(entries);
}

type SitemapEntry = { loc: string; lastmod?: string };

function xml(entries: SitemapEntry[]): Response {
  const body = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...entries.map((entry) => {
      // lastmod only when a real date exists. A lastmod that is always "now"
      // gets ignored after a few crawls, which costs more than omitting it.
      const lastmod = entry.lastmod ? `<lastmod>${escapeXml(entry.lastmod)}</lastmod>` : "";
      return `<url><loc>${escapeXml(entry.loc)}</loc>${lastmod}</url>`;
    }),
    "</urlset>",
    "",
  ].join("\n");
  return new Response(body, {
    headers: {
      "Content-Type": "application/xml; charset=utf-8",
      "Cache-Control": "public, max-age=600, s-maxage=3600",
    },
  });
}

function escapeXml(value: string): string {
  return value.replace(/[<>&'"]/g, (character) =>
    ({ "<": "&lt;", ">": "&gt;", "&": "&amp;", "'": "&apos;", '"': "&quot;" })[character] ?? character,
  );
}
