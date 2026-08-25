import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { extractTenantSlug, isUsableTenantSlug, platformBaseHostname } from "@/lib/tenant-host";

const PUBLIC_PATHS = [
  "/login",
  "/register",
  "/sign-in",
  "/sign-up",
  "/forgot-password",
  "/reset-password",
  "/accept-invitation",
  "/api/auth",
  "/apply",
  "/waitlist",
  "/track",
  "/certificate",
  // Trailing slash is deliberate: bare "/p" would also match "/pilgrim"
  // (startsWith), wrongly making the authenticated pilgrim app public.
  "/p/",
  "/firebase-messaging-sw.js",
];

// Every /register, /apply, /waitlist route already takes operatorId (and,
// for register/waitlist, seasonId) as path segments — a tenant subdomain
// (vacana.tawafiqhub.id/register/musim-haji-2026) rewrites onto that same
// route with the resolved operatorId/seasonId injected, so the page
// components underneath need zero changes. Routes in SEASON_AWARE_ROUTES
// additionally resolve a bare request (no season segment at all) to the
// operator's current active season, and treat any segment that IS present
// as a season *slug* rather than a raw ID — nothing UUID-shaped ever
// appears in a URL a visitor sees.
const SUBDOMAIN_ROUTES = ["/register", "/apply", "/waitlist"];
const SEASON_AWARE_ROUTES = new Set(["/register", "/waitlist"]);

function tenantUrl(request: NextRequest, slug: string, pathname: string) {
  const target = request.nextUrl.clone();
  target.hostname = `${slug}.${platformBaseHostname(target.hostname)}`;
  target.pathname = pathname;
  return target;
}

// In-memory, per-process — same tradeoff as every other cache in this
// codebase (operatorCacheTTL, MyAccess cache, ...), fine for the
// single-instance deployment. Keeps Resolve*Slug (called on every subdomain
// page load) from hitting the API on every single request.
const CACHE_TTL_MS = 5 * 60_000;

async function connectFetch<T>(procedure: string, body: Record<string, string>): Promise<T | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}${procedure}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify(body),
    });
    if (!response.ok) return null;
    return (await response.json()) as T;
  } catch {
    return null;
  }
}

const operatorCache = new Map<string, { operatorId: string | null; activeSeasonId: string | null; expiresAt: number }>();

async function resolveOperator(slug: string): Promise<{ operatorId: string | null; activeSeasonId: string | null }> {
  const cached = operatorCache.get(slug);
  const now = Date.now();
  if (cached && cached.expiresAt > now) return cached;

  const data = await connectFetch<{ operatorId?: string; activeSeasonId?: string }>(
    "/hajj.v1.OperatorService/ResolveOperatorSlug",
    { slug },
  );
  const result = {
    operatorId: data?.operatorId || null,
    activeSeasonId: data?.activeSeasonId || null,
  };
  // A network/API error must not cache a false negative for the full TTL —
  // the next request should try again immediately.
  if (data) operatorCache.set(slug, { ...result, expiresAt: now + CACHE_TTL_MS });
  return result;
}

const seasonCache = new Map<string, { seasonId: string | null; expiresAt: number }>();

async function resolveSeason(operatorId: string, seasonSlug: string): Promise<string | null> {
  const key = `${operatorId}:${seasonSlug}`;
  const cached = seasonCache.get(key);
  const now = Date.now();
  if (cached && cached.expiresAt > now) return cached.seasonId;

  const data = await connectFetch<{ seasonId?: string }>("/hajj.v1.SeasonService/ResolveSeasonSlug", {
    operatorId,
    slug: seasonSlug,
  });
  const seasonId = data?.seasonId || null;
  if (data) seasonCache.set(key, { seasonId, expiresAt: now + CACHE_TTL_MS });
  return seasonId;
}

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  const slug = extractTenantSlug(request.headers.get("host") ?? "");

  // /p/{slug} was the original public URL. Keep old bookmarks working, but
  // make the tenant subdomain root the one canonical address visitors see.
  const legacyProfile = pathname.match(/^\/p\/([a-z0-9](?:[a-z0-9-]*[a-z0-9])?)\/?$/);
  if (!slug && legacyProfile?.[1] && isUsableTenantSlug(legacyProfile[1])) {
    return NextResponse.redirect(tenantUrl(request, legacyProfile[1], "/"), 308);
  }

  // The public profile remains implemented by app/p/[slug], while visitors
  // see the cleaner operator root URL (vacana.tawafiqhub.id/).
  if (slug && pathname === "/") {
    const rewritten = request.nextUrl.clone();
    rewritten.pathname = `/p/${slug}`;
    return NextResponse.rewrite(rewritten);
  }

  // Tenant editorial pages stay on the tenant hostname while rendering from
  // the same public profile snapshot as the homepage.
  if (slug && (/^\/(blog|berita)\/[^/]+\/?$/.test(pathname))) {
    const rewritten = request.nextUrl.clone();
    rewritten.pathname = `/p/${slug}${pathname}`;
    return NextResponse.rewrite(rewritten);
  }

  const subdomainRoute = slug && SUBDOMAIN_ROUTES.find((route) => pathname === route || pathname.startsWith(`${route}/`));
  if (slug && subdomainRoute) {
    const { operatorId, activeSeasonId } = await resolveOperator(slug);
    if (!operatorId) {
      return new NextResponse("Operator not found", { status: 404 });
    }

    let rest = pathname.slice(subdomainRoute.length); // "" or "/some-slug" etc.

    if (SEASON_AWARE_ROUTES.has(subdomainRoute)) {
      if (rest === "" || rest === "/") {
        if (!activeSeasonId) {
          return new NextResponse("No active season", { status: 404 });
        }
        rest = `/${activeSeasonId}`;
      } else {
        const seasonSlug = rest.slice(1).split("/")[0] ?? "";
        const seasonId = await resolveSeason(operatorId, seasonSlug);
        if (!seasonId) {
          return new NextResponse("Season not found", { status: 404 });
        }
        rest = `/${seasonId}${rest.slice(seasonSlug.length + 1)}`;
      }
    }

    const rewritten = request.nextUrl.clone();
    rewritten.pathname = `${subdomainRoute}/${operatorId}${rest}`;
    return NextResponse.rewrite(rewritten);
  }

  const isPublic =
    pathname === "/" || PUBLIC_PATHS.some((path) => pathname.startsWith(path));

  if (isPublic) {
    return NextResponse.next();
  }

  const sessionToken =
    request.cookies.get("better-auth.session_token") ??
    request.cookies.get("__Secure-better-auth.session_token");
  if (!sessionToken) {
    return NextResponse.redirect(new URL("/sign-in", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|icons|manifest.json|sw.js).*)"],
};
