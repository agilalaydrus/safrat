import { tenantFromHost } from "@/lib/tenant-from-host";

export const dynamic = "force-dynamic";

type Season = { slug?: string; id?: string; endDate?: string };
type Article = { slug?: string; publishedAt?: string };
type Profile = {
  slug?: string;
  activeSeasons?: Season[];
  content?: { news?: Article[]; blogPosts?: Article[] };
  updatedAt?: string;
};

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

  const profile = await getProfile(tenant.slug);
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
      entries.push({ loc: `${base}/${path}/${article.slug}`, lastmod: article.publishedAt });
    }
  }
  return xml(entries);
}

type SitemapEntry = { loc: string; lastmod?: string };

async function getProfile(slug: string): Promise<Profile | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/GetPublicProfile`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ slug }),
      cache: "no-store",
    });
    if (!response.ok) return null;
    return (await response.json()) as Profile;
  } catch {
    return null;
  }
}

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
