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
  manual testing. Migrations **073–083 are applied locally**; in prod goose
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
