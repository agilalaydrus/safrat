# Handoff Notes

> Working state + prioritized roadmap for the next agent. Point-in-time snapshot
> (2026-08-24). Verify against current code before trusting any file:line.

## Owner workflow preferences

- After verified implementation work, always create a local commit so progress
  is recorded. Never push or deploy unless the owner explicitly asks.
- After every commit, make sure the local development server is running again
  and verify the web endpoint so the owner can immediately inspect the result.
- Every handoff response must include: a concise summary, completed work,
  remaining/unverified work, recommendations, local commit, and server status.

## Continuation after this snapshot

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
- The offline continuation is committed locally as `c76f460`; nothing has been
  pushed or deployed.

## Repo / deploy state

- **16 commits sit on local `main`, NOT pushed** after the hardening continuation
  in the current work (see `git log origin/main..main`).
  **Pushing `main` triggers a production deploy** (`.github/workflows/deploy.yml`
  → builds images, runs goose migrations, redeploys `app.tawafiqhub.id`). The
  owner has deliberately **held deploys** — do not push `main` unless told.
- Generated code (`apps/api/internal/gen`, `packages/proto-gen`) is **gitignored**
  and rebuilt by CI — never commit it. `apps/web/tsconfig.tsbuildinfo` and
  untracked scratch `*.md` / media are also excluded.
- **Local dev DB was wiped clean** (all rows truncated, schema kept) for fresh
  manual testing. Migrations **073–079 are applied locally**; in prod goose
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
