import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { PLATFORM_BASE_HOSTS, extractTenantSlug, isUsableTenantSlug, platformBaseHostname } from "@/lib/tenant-host";

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

/** True when the hostname is one the platform itself owns. */
/** Rewrites while preserving the tenant identity for the re-entered request. */
function tenantRewrite(request: NextRequest, target: URL, slug: string) {
  const headers = new Headers(request.headers);
  headers.set(TENANT_SLUG_HEADER, slug);
  return NextResponse.rewrite(target, { request: { headers } });
}

function isPlatformHost(host: string): boolean {
  const hostname = normalizedHost(host);
  return PLATFORM_BASE_HOSTS.some((base) => hostname === base || hostname.endsWith(`.${base}`));
}

function normalizedHost(host: string): string {
  return (host.split(":")[0] ?? "").toLowerCase().replace(/\.$/, "");
}

/** The platform's own apex, taken from the configured app origin. */
function platformApexHostname(): string {
  const configured = process.env.NEXT_PUBLIC_APP_URL;
  if (configured) {
    try {
      return new URL(configured).hostname;
    } catch {
      // fall through to the compiled-in default below
    }
  }
  return PLATFORM_BASE_HOSTS[0] ?? "tawafiqhub.id";
}

const TENANT_SLUG_HEADER = "x-tenant-slug";

const domainCache = new Map<string, { slug: string | null; expiresAt: number }>();

/**
 * Resolves a client's own hostname to its operator slug. Returns null for
 * platform hostnames and for anything unrecognised, so the caller falls through
 * to the normal application routing.
 */
async function resolveCustomDomainSlug(host: string): Promise<string | null> {
  const hostname = (host.split(":")[0] ?? "").toLowerCase().replace(/\.$/, "");
  // A bare hostname with no dot is never a routable client domain, and this
  // keeps localhost and internal health checks from making a lookup per request.
  if (!hostname || !hostname.includes(".")) return null;
  if (PLATFORM_BASE_HOSTS.some((base) => hostname === base || hostname.endsWith(`.${base}`))) return null;

  const cached = domainCache.get(hostname);
  const now = Date.now();
  if (cached && cached.expiresAt > now) return cached.slug;

  const data = await connectFetch<{ slug?: string }>(
    "/hajj.v1.OperatorService/ResolveOperatorDomain",
    { hostname },
  );
  const slug = data?.slug || null;
  domainCache.set(hostname, { slug, expiresAt: now + CACHE_TTL_MS });
  return slug;
}

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

  const host = request.headers.get("host") ?? "";
  // A platform subdomain still carries its slug in the hostname. Anything else
  // may be a client's own domain, which has no slug to derive and must be
  // looked up. Only verified domains resolve, so pointing a hostname at us
  // without proving ownership routes nowhere.
  // Next re-enters middleware on the path we rewrite to, and on that pass the
  // Host is the server's own address — so the tenant identity derived from the
  // hostname is gone. Carrying it in a request header keeps every later branch
  // (notably the legacy /p/{slug} redirect) seeing the same tenant it did on
  // the first pass, instead of concluding there is none.
  const forwardedSlug = request.headers.get(TENANT_SLUG_HEADER);
  // Our own rewrite, coming back through. Routing was already decided on the
  // first pass and the target is tenant content by definition, so re-running
  // the rules here can only misfire — the rewritten /p/{slug} path is not a
  // tenant route, so it would be treated as an application route and bounced
  // to the apex.
  if (forwardedSlug) {
    return NextResponse.next();
  }

  const slug = extractTenantSlug(host) || (await resolveCustomDomainSlug(host));

  // /p/{slug} was the original public URL. Keep old bookmarks working, but
  // make the tenant subdomain root the one canonical address visitors see.
  const legacyProfile = pathname.match(/^\/p\/([a-z0-9](?:[a-z0-9-]*[a-z0-9])?)\/?$/);
  // Only on a platform hostname. tenantUrl builds {slug}.{platform base}, which
  // is meaningless on a client's own domain — it would bounce a visitor off
  // umrohvacana.com onto the platform subdomain. It also collides with the
  // rewrite below: the tenant root rewrites "/" to "/p/{slug}", and if that
  // path is ever seen here without a resolved slug, this would redirect away
  // from the client's domain instead of rendering it.
  const platformHostname = isPlatformHost(host);
  if (platformHostname && !slug && legacyProfile?.[1] && isUsableTenantSlug(legacyProfile[1])) {
    return NextResponse.redirect(tenantUrl(request, legacyProfile[1], "/"), 308);
  }

  // The public profile remains implemented by app/p/[slug], while visitors
  // see the cleaner operator root URL (vacana.tawafiqhub.id/).
  if (slug && pathname === "/") {
    const rewritten = request.nextUrl.clone();
    rewritten.pathname = `/p/${slug}`;
    return tenantRewrite(request, rewritten, slug);
  }

  // Tenant editorial pages stay on the tenant hostname while rendering from
  // the same public profile snapshot as the homepage.
  if (slug && (/^\/(blog|berita)\/[^/]+\/?$/.test(pathname))) {
    const rewritten = request.nextUrl.clone();
    rewritten.pathname = `/p/${slug}${pathname}`;
    return tenantRewrite(request, rewritten, slug);
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
    return tenantRewrite(request, rewritten, slug);
  }

  // Anything still on a tenant hostname here is an application route, not
  // tenant content — every tenant route returned above. Serving these on the
  // tenant origin breaks them all: Better Auth is pinned to NEXT_PUBLIC_APP_URL
  // (the apex), its session cookie is host-only for the apex and so is never
  // sent to a subdomain, and the /sign-in fallback below is relative — so a
  // visitor lands on a tenant-origin sign-in page whose every /api/auth call is
  // blocked by CORS. /pilgrim, /leader, /agent and /dashboard are all affected.
  //
  // The target is built from the Host header, not request.url: Next normalizes
  // request.url (and nextUrl) to the address it is bound to, so the real
  // hostname survives only in the header. The Location is also written by hand,
  // because NextResponse.redirect() relativizes a Location that matches Next's
  // own origin, which would send the tenant host back to itself in a loop.
  if (slug) {
    const host = request.headers.get("host") ?? "";
    const port = host.includes(":") ? `:${host.split(":")[1]}` : "";
    const protocol = request.headers.get("x-forwarded-proto") ?? request.nextUrl.protocol.replace(":", "");
    // platformBaseHostname returns a client's own domain unchanged, which would
    // redirect that domain to itself forever. Application routes always belong
    // to the platform apex, so fall back to the configured app origin for any
    // hostname we do not own.
    //
    // TEMPORARY: this redirect exists only because Better Auth is pinned to a
    // single origin. Once sessions are issued per client domain (Level 3),
    // these routes should be served on the client's domain, not redirected.
    const derivedHost = platformBaseHostname(host);
    const apexHost = derivedHost === normalizedHost(host) ? platformApexHostname() : derivedHost;
    const apex = `${protocol}://${apexHost}${port}${pathname}${request.nextUrl.search}`;
    return new NextResponse(null, { status: 307, headers: { location: apex } });
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
  // Keep public image assets out of tenant/auth routing. Without this,
  // /images/* is treated as an application page on a tenant subdomain and
  // redirects to /sign-in instead of returning the actual image file.
  matcher: ["/((?!_next/static|_next/image|images|favicon.ico|icons|manifest.json|sw.js).*)"],
};
