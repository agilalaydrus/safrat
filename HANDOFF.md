# Handoff Notes

> Working state + prioritized roadmap for the next agent. Point-in-time snapshot
> (2026-08-26). Verify against current code before trusting any file:line.

## Owner workflow preferences

- After verified implementation work, always create a local commit so progress
  is recorded. Never push or deploy unless the owner explicitly asks.
- After every commit, make sure the local development server is running again
  and verify the web endpoint so the owner can immediately inspect the result.
- Every handoff response must include: a concise summary, completed work,
  remaining/unverified work, recommendations, local commit, and server status.

## Continuation after this snapshot

### Billing — operator subscriptions (2026-08-26)

Until this, `plan` was a label with nothing behind it: prices were published but
nothing could charge for them, and the only way to change a plan was UPDATE by
hand. Shipped in four commits: `2d113c5` schema, `a5f8e45` the gate, `ae9cae5`
the operator-facing screen, `35ba085` gateway settlement.

**Rules that matter**

- New operators get a **3-day trial**. Existing operators were seeded ACTIVE
  with 90 days — they are live and were never charged; a migration must not lock
  them out.
- Access is granted **by time (`access_until`), never by status**. A stale
  status can then never hand out free access.
- **Dashboard locks, everything else keeps running.** The gate sits after the
  staff session resolves, so the storefront, registration, and the pilgrim,
  leader and agent portals are untouched. Pressure lands on the operator who
  owes money, not on pilgrims who already registered.
- Billing procedures stay reachable while locked; everything else fails closed.
  A missing subscription row is *allowed* — locking a paying customer out over a
  missing row is the worse failure.
- The screen lives **outside the dashboard shell** because the shell loads gated
  data on mount and would not render for a locked operator.

**Bank transfer matching**

The unique amount suffix is the only thing tying a mutation to an invoice, so
uniqueness is enforced by a partial unique index over unpaid transfers — not by
checking before inserting, which would hand two simultaneous requests the same
amount. A test drives 40 concurrent issuances. `FindPayableByAmount` is written
and tested but **has no caller yet**: bank transfers are still settled by hand.

Before building a scraper, consider an aggregator with a real API (Moota,
Mutasibank). A scraper fails *silently* — the operator has paid, the system does
not know, and they stay locked out. The code shape is identical either way:
`FindPayableByAmount` then `MarkPaid`.

**Two traps found while building this**

1. The gate first landed in the `sessionOnlyProcedures` branch, which would have
   locked out **tour leaders** instead of operators. A compile error caught it.
   The anchor is now asserted unique so the slip cannot recur silently.
2. `err?.code === "unauthenticated"` in the dashboard shell **never worked**:
   Connect-ES exposes the code as a numeric enum, so the string comparison is
   always false. The new redirect uses `Code.FailedPrecondition`; the pre-existing
   string checks alongside it are still wrong and worth a separate fix.

**Do not rely on bank transfer in production until matching exists** — the
operator will pay and stay locked out. QRIS/card is safe: verified end to end
against the running API.

### Session log — 2026-08-26


**Pricing tiers map onto the white-label levels** (owner, 2026-08-26). The
`plan` enum from migration 001 already matches them:

| Plan | Tier | Entitlement |
| --- | --- | --- |
| `STARTER` | Starter PPIU | Platform subdomain. Landing page still fully customisable, and pilgrim / muttawwif / agent portals all included. |
| `GROWTH` | Growth Enterprises | Everything in Starter plus their own domain. |
| `PRO` | Custom Enterprises | Custom, up to a separate VPS and bespoke development. Open to any travel agency, not only PIHK or konsorsium — the tier is defined by what it offers, not by who buys it. |

**`plan` was decorative until now** — the column existed since 001 but was read
nowhere in the Go code, so no feature was gated by it. Custom domains are the
first entitlement actually enforced. If other paid features are assumed to be
gated, check: they probably are not.

Enforcement is at **resolution**, not only at creation: a verified domain stops
resolving and drops out of the CORS allowlist the moment the plan no longer
includes it. Gating only on create would leave a downgraded operator's domain
served forever. `ListMyDomains` returns `customDomainsEnabled` so the CMS can
explain the entitlement, but the server stays the authority.

**White-label foundation (Level 2)**

The owner's direction: clients will eventually bring their own domains, and
possibly their own VPS, with jamaah / tour leader / agent all signing in on the
client's domain. Levels: 1 = platform subdomain (where we were), 2 = client
domain for public pages with auth still on the apex (where we are now), 3 =
sessions issued per client domain, 4 = dedicated instance per client.

Shipped:
- **Migration 085 `operator_domains`** (`19ebaf7`). Tenancy used to be *derived*
  from the hostname; a client domain has no slug to derive, so identity is now
  stored. Additive by design — platform subdomains still resolve exactly as
  before and nothing was backfilled. Only verified rows resolve, enforced in
  SQL and in the repository.
- **Dynamic CORS** (`35b9dd8`) reading the same table, since `/register` and
  `/apply` are served on client domains and call the API from the browser.
- **Domain claim + DNS TXT verification** with a settings panel (`7b8b694`).

Three routing traps found only by testing against the standalone server, none
visible by reading the code:
1. `platformBaseHostname` returns a client domain unchanged, so the app-route
   redirect would have sent it to itself forever.
2. The legacy `/p/{slug}` redirect bounced visitors off the client domain onto
   the platform subdomain.
3. Next re-enters middleware on the rewritten path with the server's own Host,
   losing tenant identity — now carried in a request header.


**Caddy edge is built but NOT cut over** (`b901901`). `deploy/caddy/` contains
the Caddyfile, an install script, and a cutover runbook. nginx is still the live
edge; the swap changes what terminates TLS for every tenant, so it is manual and
deliberate rather than part of a deploy.

Why Caddy at all: the wildcard comes from lego over Hostinger DNS-01, which can
only cover domains in our own Hostinger account. A client's domain has DNS at
their registrar, so its certificate must come from HTTP/TLS-ALPN — per domain,
on first request. The existing wildcard is deliberately untouched: the lego
timer keeps renewing it and Caddy loads it from disk, so no DNS plugin is needed
and the proven path is not risked.

`/internal/tls-authorize` is what makes on-demand safe. Caddy asks before
issuing for an unseen hostname; a 200 authorises it, so the endpoint is exactly
as strict as routing. Without it, anyone pointing DNS at the server could make
it request certificates on their behalf and burn Let's Encrypt rate limits. Same
403 for unknown and not-entitled, so it cannot be used to probe plans.

Two things to carry into the cutover:
- **Certificate file permissions.** Caddy runs as the `caddy` user;
  `/etc/letsencrypt/live/*/privkey.pem` is usually root-only. Check with
  `sudo -u caddy test -r ...` *before* stopping nginx, or Caddy fails to start
  at the exact moment there is nothing serving.
- **X-Real-IP.** The Go rate limiter trusts it because the proxy always sets it
  itself. Caddy does not by default; the Caddyfile sets it from `{remote_host}`.
  Any future edge change must preserve that, or per-IP limits become spoofable.

Rollback is a service swap (`stop caddy && start nginx`), not a restore — nginx
keeps its config throughout. `deploy.yml` still installs the nginx config and
must only be switched to `install-caddy` after Caddy is live.

**Still open, in order**
- **Caddy + on-demand TLS.** The wildcard is issued by lego + Hostinger DNS-01,
  which cannot work for domains at a client's registrar; those need HTTP-01.
  Plan: Caddy terminates TLS, loads the existing wildcard from disk (so the
  proven renewal path is untouched) and issues on-demand certificates for
  client domains, gated by an `ask` endpoint backed by `operator_domains`.
  Deliberately a separate migration, not bundled with feature work.
- **Level 3 auth.** Better Auth is pinned to one origin and its cookie is
  host-only for the apex. `trustedOrigins` supports an async per-request
  function in 1.6.28, but was deliberately NOT widened yet — auth never happens
  on a client domain today, so trusting those origins would be risk without use.
- **Centralised OAuth handoff.** Google requires every redirect URI to be
  registered manually, so social login cannot happen directly on arbitrary
  client domains. One central callback issues a single-use, short-lived,
  origin-bound code that the client domain exchanges server-side. Never put a
  session token in a URL.
- **The app-route redirect is TEMPORARY** and inverts at Level 3. It is marked
  as such in `middleware.ts`.

**Also outstanding (user-side audit)**
- No `manifest.json` or icons: the pilgrim/leader PWAs cannot be installed.
- `favicon.ico` 404s on every origin; `<html lang="en">` on an Indonesian UI;
  `/pilgrim`, `/leader`, `/agent` all inherit the title "Operator Dashboard".

**Later the same day — storefront UI round**

- **The tenant header was never sticky.** `.tenant-scope` carried
  `overflow-x: hidden`, which forces `overflow-y` to compute as `auto` and makes
  the element a scroll container; `position: sticky` then anchored the header to
  that box instead of the viewport, so it scrolled away with the page. Measured
  at `scrollY 2500` the header sat at viewport top `-2500`. Fixed with
  `overflow-x: clip` (`2a0b83d`), which contains horizontal overflow without
  creating a scroll container. Nothing in the CSS *looked* wrong, so
  `storefront-sticky-nav.spec.ts` asserts the header's measured bounding rect
  and pins `overflow-y: visible` — reintroducing `hidden` now fails loudly.
  Any future `overflow` change on `.tenant-scope` must keep this in mind.

- **Packages section reworked** (`6018cd7`): a carousel of 4:5 poster cards
  (operators upload portrait promo flyers, so the artwork is shown whole rather
  than cropped through the price), plus a picker panel opened by a card or the
  floating shortcut — dark header, scrollable departures, expandable rows
  carrying the facilities/hotels/airline/room-price detail that previously hid
  behind a `<details>` element. Built entirely from existing proto fields; no
  CMS or schema change. Carousel arrows only render when the track overflows.

- **Contrast constraint, worth repeating:** `--tenant-brand-text` comes from
  `readableText`, a binary luminance flip calibrated against `--tenant-brand`
  exactly. A gradient that darkens below the brand colour destroys it — a first
  attempt at the contact panels did precisely that and made the copy nearly
  unreadable. Panels now only ever lighten. Screenshot the result; reading the
  CSS will not catch this.

- **Reminder about the local PWA server:** `pnpm build` overwrites
  `public/sw.js` and breaks the offline project until `e2e:pwa:build` is re-run.
  That failure mimics a real regression exactly.

Everything below shipped to production in four deploys, all green. Ordered by
what a reader most likely needs to know first.

**Live in production**

| Commit | What |
| --- | --- |
| `6224056` | Storefront media quota + asset registry (migration 084) |
| `adf392b` | `cmd/storefront-backfill` for pre-registry objects |
| `9168a23` | Ship the backfill binary in the API image |
| `8e0cfbf` | Grant the MinIO service user `s3:ListBucket` |
| `6cae083` | Tenant storefront auth links point at the apex |
| `732f755` | All app routes on a tenant host redirect to the apex |
| `dad1080` | Nav hover, floating CTA, and contact panel polish |
| `40b241b` | Playwright browser E2E suite (`apps/web/e2e/`) |
| `1086d7e`, `93af574` | Doc corrections |

**Closed**

- Browser QA now exists and passes (10 specs). See `apps/web/e2e/README.md`.
- The MinIO production rollout was already done in the `e7fef46` release; the
  earlier "still pending" note here was stale.
- The backfill ran in production and found **nothing to adopt** — the bucket is
  empty. `-apply` is not outstanding work.
- Tenant sign-in via the storefront's own links works; confirmed by the owner in
  a real browser, including a successful Google sign-in.

**Known gaps, deliberately open**

- **Real-device QA.** iOS Safari audio autoplay, and an installed PWA reopened
  offline, are still unverified. No amount of headless testing settles these.
- **The dashboard lives on the apex, by design.** `tawafiqhub.id/dashboard`, not
  `vacana.tawafiqhub.id/dashboard`. Better Auth is pinned to one origin and its
  cookie is host-only for the apex. Moving the dashboard onto tenant subdomains
  means cross-subdomain cookies, multi-origin CORS, and Better Auth changes —
  an architecture decision, not a fix. Only worth doing if full white-label is
  a business goal.
- **`readableText` is a binary luminance flip** (`TenantStorefront.tsx`). For an
  emerald brand it returns dark navy, which is only ~4.5:1 on the brand fill and
  drops below AA wherever the design applies opacity. Any future panel styling
  must not darken below `--tenant-brand`. Raising the threshold would improve
  contrast for everyone but flips existing tenants' text colour, so it needs a
  deliberate decision.
- **Native mobile apps.** `apps/mobile-leader` and `apps/mobile-pilgrim` are
  still empty scaffolds; modules 5/6 ship as PWAs.

**Two mistakes worth not repeating**

Both were the same error: verifying against a *more privileged or more
convenient* path than production uses.

1. The local API had **no `S3_*` keys**, so `storage.New` returned nil and every
   upload RPC was silently a no-op. The MinIO integration tests passed because
   they construct a `Store` directly, bypassing the API and the browser.
2. The backfill was "verified end-to-end locally" using MinIO's **root**
   credentials, which bypass the bucket policy — so the missing `s3:ListBucket`
   only surfaced as a 403 in production.

The fix in both cases was to reproduce the real conditions: real env, real
least-privilege user, real browser. Prefer that over a convenient approximation.

- **The storefront backfill is done, and there was nothing to adopt.** Run in
  production on 2026-08-26 after the deploy of `8e0cfbf`: `objects scanned: 0`,
  everything else 0. Confirmed independently with MinIO root credentials (which
  bypass the bucket policy) — `safrat-uploads` holds 0 objects, 0 bytes. No
  operator had uploaded storefront media yet, because the storefront's default
  logo/hero/gallery images are bundled Next assets served from `/images/...`,
  not objects in the bucket. `-apply` was therefore never needed and is not
  pending: every future upload is registered by the normal path. The command
  stays in the image as re-runnable maintenance, not as outstanding work.

- **The backfill's first production run failed with a 403 on ListObjectsV2.**
  The least-privilege MinIO service user had no `s3:ListBucket`. Fixed in
  `8e0cfbf` by granting it, conditioned to the `storefront/` prefix. The cause
  of the miss is worth remembering: local runs used the *root* credentials from
  `docker-compose.yml` (`safrat-local`), which bypass the policy, so the real
  permission path was never exercised. Verified the second time by creating a
  MinIO user, attaching the actual policy file, and confirming the old policy
  reproduces the exact 403 while the new one lists correctly.

- **Tenant sign-in was broken by CORS and is partly fixed** (`6cae083`). The
  tenant storefront linked to `/sign-in` relatively, so on
  `vacana.tawafiqhub.id` the page rendered on the tenant origin while Better
  Auth's client stayed pinned to `NEXT_PUBLIC_APP_URL` — every `/api/auth` call
  cross-origin and blocked. All four auth links are now absolute to the apex.
  **Still broken:** typing a tenant `/sign-in` URL directly. That needs a
  cross-host redirect from middleware, which could not be verified locally
  (Next normalizes the request host to localhost in dev and collapses even a
  genuinely cross-origin `Location` down to a bare path, looping the tenant host
  back to itself). Shipping it unverified risked looping every tenant
  storefront, so it was deliberately left out. **Browser confirmation of the
  shipped fix on a real tenant subdomain is still outstanding.**

- **Browser QA now exists.** `apps/web/e2e/` is a Playwright suite that drives
  the real local stack (Next.js, the Go API, PostgreSQL, MinIO) through a real
  browser — see `apps/web/e2e/README.md` for how to run it and what it covers.
  Eight specs pass: the full image-upload chain (presign → WebP conversion →
  PUT → verification → promotion → registry row → public read → quota), draft
  vs published isolation, a real MP3 upload with autoplay and mute persistence,
  the blocked-autoplay fallback, service worker install/precache, and a fresh
  tab served entirely from cache with the network down. It is deliberately
  outside `pnpm lint`/`typecheck`/CI because it needs running services and
  writes to the local database. Fixtures are a dedicated operator and a linked
  pilgrim, provisioned through the app's own endpoints; `e2e:clean` removes them.
  Known limits are listed in that README — autoplay blocking is injected rather
  than enforced by the browser, the cold-start test keeps one tab open, and real
  devices (iOS Safari audio, an installed PWA reopened offline) remain unverified.

- **The local API had storefront storage disabled entirely.** `apps/api/.env`
  carried no `S3_*` keys, so `storage.New` returned nil and every upload RPC was
  a no-op locally; the MinIO integration tests pass because they construct a
  Store directly and bypass the API. The keys are now set (they were already
  documented in `.env.example`), which is what made browser upload QA possible
  at all.

- **Found by that QA:** the storefront quota indicator is stale after an upload.
  `setStorageUsage` in `OperatorProfilePanel` is called from load, saveDraft and
  publish, never from the upload handler, so "N file aktif" and the used-bytes
  bar do not move until the operator saves, publishes, or reloads. Not a data
  bug — the registry and quota enforcement are correct — but the operator sees
  the wrong number for the rest of the session. `storefront-cms.spec.ts` asserts
  the current behaviour and documents it.

- The one-time inventory/backfill for pre-registry storefront objects now
  exists as `apps/api/cmd/storefront-backfill`. It pages the live `storefront/`
  prefix, resolves each key to its operator and asset kind, and adopts the
  objects into the registry as LIVE rows so their bytes count toward the
  operator quota and the cleanup worker can manage them. It only reads the
  bucket and inserts rows — it never uploads, moves, or deletes an object, and
  never modifies an existing row, so it is safe to re-run. It defaults to a dry
  run whose counts are exact, because the real statement runs in a transaction
  that is rolled back; `-apply` commits. Objects whose operator no longer
  exists, whose size is out of the registry's bounds, or whose key does not
  parse are counted and reported rather than silently skipped. **Adopting an
  object brings it under the cleanup sweep**, so an adopted object referenced by
  neither the draft nor the published snapshot is deleted after the seven-day
  recovery window — run the dry run first and read the counts. Verified
  end-to-end against local MinIO plus PostgreSQL: dry run wrote nothing,
  `-apply` adopted the object with its real size, a second `-apply` adopted
  nothing and left quota usage unchanged, and a deliberately unparsable key was
  reported and left alone. The local database and bucket were returned to their
  prior state afterwards.

- The S3 environment resolution now lives once in `storage.ConfigFromEnv()`.
  `cmd/worker` previously had its own copy that silently dropped the legacy
  `R2_*` fallbacks `config.Load()` honours; the server, the worker, and the
  backfill command now all read the same variables the same way.

- Storefront media hardening now has a PostgreSQL-backed reservation and live
  asset registry (migration 084), enforcing a configurable per-operator quota
  under an advisory transaction lock so concurrent uploads across API replicas
  cannot overrun the limit. Confirmation records the verified live object and
  stays retry-safe if PostgreSQL is temporarily unavailable. The CMS reports
  used/quota bytes, active files, and pending uploads. An hourly worker expires
  stale reservations, marks files unused only when absent from both draft and
  published snapshots, waits a seven-day recovery window, then removes the
  object before its registry row. The worker has the same least-privilege S3
  configuration as the API. Migration 084 and its quota/reference integration
  tests pass on the local PostgreSQL database; worker ordering/error tests,
  storage tests, full Go test/vet/build, frontend lint/typecheck/build, Buf lint,
  and Compose validation also pass. Existing pre-registry objects remain safe
  and unmanaged; a one-time inventory/backfill is recommended later if exact
  historical usage accounting is required.

- Storefront customization now also covers the previously hard-coded details:
  operators can upload a dedicated WebP About image with required alt text and
  caption, edit exactly four unique assurance pillars, define one to seven
  unique operating-hour groups, and choose an initial music volume from 5 to
  100 percent. Existing tenant snapshots receive compatible defaults in the
  service layer, so old published pages continue rendering without migrations.
  The public renderer uses the custom values while retaining safe bundled-image
  fallbacks. API and CMS validation agree on counts, required values, duplicate
  prevention, URL safety, and volume limits.

- Berita and Blog now have a complete draft editor: collapsible article forms,
  automatic but overridable slugs, tenant-scoped WebP cover uploads, required
  alt text, author, publication date, excerpt/body counters, per-article SEO
  title and description, and a live Google-result preview. Client and backend
  validation enforce complete articles, valid 3 to 180 character slugs, unique
  slugs across both collections, valid cover URLs, alt text, valid timestamps,
  and a maximum of 30 entries per collection. Published cards and detail pages
  show author/date metadata. Article media uses a dedicated storage kind while
  retaining the existing tenant isolation, verification, and 5 MB WebP limit.
  TypeScript, targeted ESLint, Buf lint, production web build, full Go tests,
  vet, build, and the new storage coverage pass. Signed-in CMS interaction and
  a real image upload remain recommended browser QA.

- The tenant storefront now has a transparent hero-overlay header that remains
  sticky, resolves to the tenant's semantic surface after the hero, highlights
  the active anchor with a rounded state, and exposes an accessible mobile menu.
  A floating package shortcut only appears after leaving the hero. Operators can
  configure up to eight unique social channels in the CMS, rendered as a
  theme-aware social hub. They can also upload one tenant-scoped MP3 up to 10 MB,
  enable looping background music, and set its title. Image uploads remain WebP
  capped at 5 MB. The API verifies MIME, size, tenant key, extension, and WebP or
  MP3 signature before promoting pending media. The storefront attempts autoplay,
  falls back after browser interaction when blocked, provides play/mute controls,
  and stores the visitor's mute preference locally. Production build, ESLint,
  Buf lint, storage tests, service tests, and the full Go suite pass. Interactive
  browser automation remains unavailable, so signed-in CMS and device-level
  audio autoplay QA remain recommended before production push.

- The tenant storefront now has a full CMS in Dashboard Settings with separate
  draft and published JSON snapshots, optimistic revision checks, authenticated
  preview, and an atomic publish operation. Public tenant pages only read the
  published snapshot, while preview uses the exact same renderer as production.
  Operators can edit brand/hero/contact content, package photos and descriptions,
  facilities, itinerary, gallery with required alt text, testimonials, and FAQ.
  Package choices remain tied to active operational seasons, so CMS content cannot
  invent or duplicate a season.
- Storefront media uploads now go directly from the browser to an S3-compatible
  bucket through a tenant-scoped, 10-minute presigned PUT. The browser redraws
  images to strip metadata, resizes them, and creates WebP before upload; the API
  then HEADs, downloads, signature-checks, fully decodes, and dimension-checks the
  object before returning its usable public URL. Local development uses MinIO on
  `:9000` (console `:9001`) with tested browser CORS. Production is now designed
  around self-hosted MinIO on the existing VPS rather than Cloudflare R2. The
  versioned Compose/bootstrap/Nginx setup uses a persistent volume, a separate
  least-privilege API user, global apex-only CORS, public reads only under
  `storefront/`, and no exposed admin console. Upload tickets target the
  `storefront-pending/` prefix and confirmation promotes verified images to
  `storefront/`; a 1-day lifecycle can therefore remove abandoned uploads
  without ever expiring published media. The real production-isolated MinIO
  integration passed CORS, signed upload, full WebP verification, promotion,
  anonymous published read, pending privacy, cleanup, and repeatable bootstrap.
  VPS secrets and the first production rollout still need to be completed.
- Migration 082 creates `operator_storefronts` and seeds every existing operator's
  legacy public profile into draft and published revision 1. The migration is
  applied locally. Repository integration tests prove draft isolation, atomic
  publish, and stale-tab conflicts; storage integration tests prove real presign,
  CORS preflight, WebP upload, verification, and cleanup against MinIO. The QA
  fixture cleanup ordering was corrected and the local database is empty again.
- The rich tenant renderer keeps TawafiqHub attribution and adds package details,
  itinerary, gallery, testimonial, and FAQ sections while retaining brand-driven
  light/dark styling. Local tenant-host smoke testing returned HTTP 200 and found
  all representative CMS sections in server-rendered HTML. In-app browser
  automation was unavailable in this session, so a signed-in visual interaction
  pass for CMS editing/upload/preview remains recommended.
- The tenant subdomain root is now a full white-label travel storefront rather
  than a compact public-profile card. Migration 081 adds one brand color plus
  editable hero eyebrow/title/subtitle/image fields; the existing operator
  name, logo, description, contact, legal details, and future seasons populate
  the remaining template. Dashboard settings exposes all editable values, the
  storefront derives accessible foreground contrast from the chosen color,
  supports light/dark mode, keeps package registration slugs unchanged, and
  permanently attributes "Powered by TawafiqHub". A 97 KB default Umrah hero
  WebP is bundled for operators without photography. Migration 081 is applied
  locally and the local database was returned to its prior empty state after a
  non-persisting storefront QA fixture. Production rollout remains pending an
  explicit push/deploy request.

- The platform apex `https://tawafiqhub.id` is the canonical app origin.
  Root/www TLS and DNS reach the VPS; the version-controlled nginx config
  proxies the apex and permanently redirects the old `app` host. The promotion
  helper owns the two historical active VPS targets (`tawafiqhub` and the now
  neutral `tawafiqhub-root`) with validation and two-file rollback. The deploy
  workflow also smoke-tests public apex, service worker, API, exact redirects,
  and CORS so inactive-config regressions cannot produce a green deployment.
  Better Auth, CORS, build URLs, and deployment defaults use the apex. The VPS
  deploy script re-exports the canonical CORS origin after sourcing `.env.prod`
  so a stale persisted `app` value cannot override the compose default. The VPS
  already has the corrected root-owned `safrat-install-nginx` helper and its
  single-command sudoers rule.
  Before production rollout, add
  `https://tawafiqhub.id/api/auth/callback/google` to the Google OAuth client's
  authorized redirect URIs. Existing host-only sessions on `app` will require
  a one-time sign-in on the apex.
- The hardening continuation fixes all proto RPC request naming violations;
  `buf lint` is now clean while RPC paths and field numbers stay wire-compatible.
- Group-city (both admin and Muttawwif entry points), kloter-status, and ritual
  bulk producers now commit their authoritative writes and outbox events in one
  PostgreSQL transaction. Migration 079 adds 30-second worker leases and bounded
  exponential retry backoff to prevent concurrent duplicate dispatch.
- Firebase push methods return errors and retry transport failures immediately
  (100ms/250ms backoff within a 4-second budget). SOS stays on the direct path;
  alert persistence never rolls back if push ultimately fails, and Sentry records
  the failure. Outbox delivery now receives push errors and can retry correctly.
- Redis now backs the shared operator cache and distributed token-bucket rate
  limiter in addition to monitoring pub/sub. Operator updates publish cross-replica
  L1 invalidations; Redis failure falls back to PostgreSQL/local limiting.
- All 23 historical React Hook warnings were resolved with stable callbacks and
  memo dependencies. ESLint now reports 0 errors and 0 warnings.

- The cold-start offline item below is committed as `c76f460` with Serwist 9:
  `app/sw.ts` is the source and production builds generate the ignored
  `public/sw.js`. All 20 `/pilgrim` + `/leader` routes and build assets are in
  the precache manifest.
- Firebase Messaging is bundled into that same production root-scope worker;
  `/firebase-messaging-sw.js` remains a development-only fallback. This fixes
  the prior collision where the cache worker and Firebase worker replaced each
  other at scope `/`.
- `RequireAccess` now has a bounded 72-hour offline access snapshot, and leader
  groups are read-through cached. Without these, a precached shell still
  redirected to sign-in during a cold offline start.
- Operator slugs now use the meaningful full name after removing generic legal
  prefixes (`PT`, `CV`, `KBIH/KBIHU`, etc.). New onboarding lets the owner edit
  that suggestion, previews `{slug}.tawafiqhub.id`, and checks availability in
  real time. Migration 076 repairs only existing generic slugs such as `pt`; it
  is applied locally.
- The operator-subdomain root is now the canonical public profile URL. The old
  `/p/{slug}` address permanently redirects to `{slug}.tawafiqhub.id/`; share
  buttons use the tenant URL, and package CTAs use season slugs instead of UUIDs.
  API/database uniqueness remains the final race-safe guard for chosen slugs.
  The wildcard continuation is implemented locally: shared frontend hostname
  parsing, reserved platform slugs, migration 080's database constraint,
  wildcard Nginx routing, automated Hostinger DNS-01 certificate renewal, and
  deploy smoke tests. It is intentionally not on production yet. Hostinger DNS
  `A * -> 103.179.66.25` is now active and verified through Cloudflare, Google,
  and both authoritative nameservers. Before any push to `main`, bootstrap the
  root-only `lego` certificate/timer from a staging worktree and reinstall the
  updated Nginx helper in the exact order documented in `DEPLOY.md`.
  ACME prechecks timed out despite both TXT values being confirmed on both
  Hostinger authoritative nameservers plus Cloudflare and Google. The renewal
  script now uses lego v5's fixed 120-second DNS wait, bypassing that false
  negative before Let’s Encrypt performs the authoritative validation.
  The apex plus wildcard certificate was issued successfully on the VPS on
  2026-08-24, its key/SAN/expiry checks passed, and the daily
  `safrat-wildcard-tls.timer` is active. Production wildcard routing still
  requires the reviewed release to reach `main` so the deployment workflow can
  promote the version-controlled Nginx configuration.
  Local verification passed: migration 080 plus a non-persisting reserved-slug
  constraint probe, all Go tests/vet/build, web lint/typecheck/production build,
  `buf lint`, shell syntax, Dockerized `nginx -t`, and Host-header routing
  (`tenant-probe.localhost` 404; legacy `/p/tenant-probe` 308).
- Landing hero messaging now sells one end-to-end operational control surface
  from Indonesia to Saudi, with gold-gradient emphasis and off-white dark-mode
  headings. FAQ dark mode separates active questions in warm gold from muted
  slate answers using unlayered semantic CSS, avoiding the Tailwind cascade
  issue that left accordion cards white. Footer office coverage is DKI Jakarta
  and Kota Bekasi.
- Season creation is idempotent and protected at three layers: synchronous UI
  submit locks, backend exact-retry upsert, and a unique normalized season name
  per operator. Migrations 077–078 safely remove empty duplicates. Same-name
  rows with dependent data make the migration fail for manual merge rather
  than cascading data loss.
- Verified locally: web typecheck, ESLint (0 errors; 0 warnings), production
  build, and generated-manifest inspection (20/20 PWA
  routes present). A real-browser/device offline test is still recommended.
- The apex-domain deployment through `dfabc98` was pushed and deployed
  successfully on 2026-08-24 (GitHub Actions run `32711294125`).

## Repo / deploy state

- The production release through `e7fef46` is on `origin/main` and passed CI,
  image builds, migrations, VPS restart, and public smoke tests in Actions run
  `32916437921`.
  **Pushing `main` triggers a production deploy** (`.github/workflows/deploy.yml`
  → builds images, runs goose migrations, installs validated nginx config, and
  redeploys `tawafiqhub.id`). Do not push again without explicit owner approval.
- Generated code (`apps/api/internal/gen`, `packages/proto-gen`) is **gitignored**
  and rebuilt by CI — never commit it. `apps/web/tsconfig.tsbuildinfo` and
  untracked scratch `*.md` / media are also excluded.
- **Local dev DB was wiped clean** (all rows truncated, schema kept) for fresh
  manual testing. Migrations **073–084 are applied locally**; in prod goose
  applies them on deploy.
- Local processes: web dev on `:3131`; Go API on `:8131`. Both are expected to
  be restarted from current source after the latest local commit.

## What shipped this session (the 5 commits)

1. `f97ad06` Landing: Masuk/Daftar prioritized, demo flow removed, WA contact
   `+62 812-8303-1003`.
2. `65ea474` Fix transparent mobile menu drawer (was trapped by the header's
   `backdrop-blur` containing block).
3. `d3a1d69` **Onboarding wizard + operator public profile** — migration 073;
   `OperatorService.UpdateMyProfile` (auth) + `GetPublicProfile` (public);
   tenant-root public profile (internally rendered by `/p/[slug]`); settings
   editor + share link; post-onboarding banner.
4. `f9dc1c8` **Production hardening**:
   - **#1 Transactional outbox** (migration 074 `cascade_events`): producers
     enqueue in the same tx as the write; worker relay (`cascade:dispatch`
     `@every 10s`, `FOR UPDATE SKIP LOCKED`, dead-letter after 5 attempts)
     drains it. **Health-report BERAT push** migrated as the atomic reference.
   - **#2 Redis-backed event bus** (`internal/events/bus.go`): same interface,
     picks Redis when `REDIS_URL` set → cross-replica. In-memory path unchanged.
     `docker-compose.prod.yml` api service now sets `REDIS_URL`.
5. `472e54d` **Offline hardening**:
   - Poison-safe write queue (`lib/offline.ts`): per-item attempts + dead-letter
     so one failing item can't wedge the SOS queue; idempotency-key plumbing.
   - **SOS create idempotency** (migration 075): `idempotency_key` + partial
     unique index; `ON CONFLICT DO NOTHING` + `created` flag → replay returns the
     existing alert without re-notifying. Verified end-to-end.

## Roadmap (prioritized) — with the analysis already done

### Completed locally, pending browser verification
- **#3 Precache for cold-start offline.** Implemented in the committed
  continuation described above. The remaining validation gap is a real-browser
  test: load each PWA online once, close it, enable offline mode, then cold-open
  the installed PWA and exercise cached reads/queued writes.

### Completed in the hardening continuation
- Group-city, kloter-status, and ritual bulk cascades use the transactional
  outbox; SOS remains direct with bounded fast retry.
- Operator cache invalidation and public RPC rate limits are Redis-distributed,
  with bounded local/DB fallbacks for availability.

### Skip / already handled
- **Check-in idempotency** — redundant: `check_ins` already has
  `UNIQUE(movement_id, pilgrim_id, type)`.
- **Chat idempotency** — possible but low value (duplicate message only).

## CI note
- `buf lint` is clean. Request message names are now method-specific; generated
  clients must be rebuilt with `pnpm buf:generate` (CI already does this).

## Local verify recipe
- Go: `cd apps/api && go build ./... && go vet ./... && go test ./...`
- Web: `pnpm --filter @hajj-saas/web typecheck && (cd apps/web && npx eslint .)`
- Redis cross-instance tests: `REDIS_TEST_URL=redis://localhost:6380 go test ./internal/events/ ./internal/middleware/ ./internal/repository/`
- Backend smoke tests run a throwaway server on `:8132` against the local DB
  (`PORT=8132 go run ./cmd/server`) — insert a temp operator, curl the RPC, clean up.
