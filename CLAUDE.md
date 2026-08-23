# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Hajj & Umrah Operator SaaS ("safrat") — a multi-tenant platform for tour operators to manage pilgrims, seasons, accommodation, transport, agents, and digital products. The design spec lives in `CODEX_SPEC.md`; treat it as the intended direction, not ground truth — the running code (especially auth) has already diverged from it in places (e.g. Better Auth issues opaque DB session tokens, not JWTs, despite the spec's section 5 header).

## Commands

```bash
pnpm install                 # install JS/TS workspaces
pnpm buf:generate            # regenerate Go + TS code from proto/ (needed after any .proto change)
pnpm dev                     # run all apps in parallel (turbo)
pnpm build                   # turbo run build
pnpm lint                    # turbo run lint
pnpm typecheck                # turbo run typecheck

cd apps/api && go run ./cmd/server   # run the Go API directly
cd apps/api && go run ./cmd/worker   # run the agent tier-recalculation worker (needs REDIS_URL)
cd apps/api && go test ./...          # run Go tests
cd apps/api && go test ./internal/service/ -run TestTransport   # single test
```

Local Postgres + Redis + migrations:
```bash
docker compose up -d postgres redis
goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up
```

The Go API refuses to start unless `DATABASE_URL`, `BETTER_AUTH_SECRET`, and `CORS_ALLOWED_ORIGIN` are all set (see `apps/api/internal/config/config.go`). `cmd/worker` is a separate entrypoint requiring `DATABASE_URL` and `REDIS_URL` — it runs an asynq scheduler + server with two periodic tasks: agent tier recalculation (`internal/worker/tier.go`, every 5 minutes — does **not** compute payouts, see the `TODO(payout)` in `internal/service/agent.go`) and SOS escalation (`internal/worker/sos.go`, every 1 minute — flips `ACTIVE` alerts older than 10 minutes to `ESCALATED` and pushes coordinators, per the business rule in `CODEX_SPEC.md` §7).

Go integration tests (e.g. `apps/api/internal/service/transport_test.go`) are skipped unless `TEST_DATABASE_URL` is set — they run against a real, disposable Postgres database and clean up the rows they create. Never point `TEST_DATABASE_URL` at a database you care about.

Prereqs: Bun 1.x locally (Node 20 LTS in prod), pnpm 9.x, Go 1.25+, Buf CLI, sqlc, goose, PostgreSQL 16.

## Architecture

**Two independently-generated API surfaces, one proto source of truth.** `proto/hajj/v1/*.proto` is compiled by `buf generate` (`pnpm buf:generate` / `scripts/generate.sh`) into two outputs: Go message/Connect-service code under `apps/api/internal/gen/`, and TS message/Connect-service code under `packages/proto-gen/`. Never hand-edit either `gen/` directory — edit the `.proto` files and regenerate. The web app's Connect client (`apps/web/lib/rpc.ts`) and its clients are built directly against `packages/proto-gen`.

**Go API: strict 3-layer separation** (`apps/api/internal/`):
- `handler/` — Connect RPC handlers only. No business logic, no direct DB access.
- `service/` — business logic. No HTTP/Connect types, no raw SQL.
- `repository/` — wraps sqlc-generated queries (`internal/gen/db`, generated from `db/query/*.sql` + `db/migrations/*.sql` per `sqlc.yaml`). Only place that touches SQL.

Handlers call services, services call repositories — never skip a layer or reach sideways. `main.go` wires all three per feature (operator, pilgrim, season, accommodation, transport, product, agent) and registers each as a separate Connect handler on one `http.ServeMux`.

**Auth & multi-tenancy.** Better Auth (Next.js side, `apps/web/lib/auth.ts`) owns users/sessions/organizations in Postgres. The Go API does not verify JWTs — `internal/middleware/auth.go` takes the bearer token from each request, looks up the live session row directly in the `session`/`member` tables, and derives `userID` + `organizationID` (used as `operatorID`) from it. Every service/repository call is expected to be scoped by `operatorID` pulled from context via `middleware.OperatorIDFromCtx` — queries without operator scoping are a correctness bug (multi-tenant data leak), not just a style issue. `middleware.ContextWithIdentity` exists specifically so services/tests can inject identity without going through the interceptor.

**Public (unauthenticated) RPCs are an explicit allowlist, not a default.** `internal/middleware/auth.go` wraps every Connect RPC with session validation by default; `publicProcedures` in that file is a deliberately small map of exceptions — `AgentService/ApplyAsAgent` (public `/apply/[operatorId]` page) plus the whole pilgrim-facing surface (`PilgrimAppService`'s three read methods, `SOSService/CreateSOSAlert`, `ChatService`'s two pilgrim methods), all authenticated by `app_access_code` instead of a session. Any handler reachable through that allowlist gets no identity from context — it must take its identity (`operator_id` or `app_access_code`) from the request body and validate it itself. Treat adding to this map as a security decision, not routing plumbing. Any procedure added to that allowlist should also be added to `rateLimitedProcedures` in `internal/middleware/ratelimit.go` (in-memory, per-client-IP token bucket; trusts `X-Real-IP` because nginx always sets it itself per `DEPLOY.md` §7 — never trust that header without a proxy in front setting it) — **except `SOSService/CreateSOSAlert`, deliberately never rate-limited**: throttling a safety feature is the wrong failure mode.

**Groups have two separate services on purpose.** `GroupService` is authenticated admin CRUD (list/create/update/delete groups, assign a leader) for the operator dashboard's `/dashboard/groups` page; `GroupLeaderService` is the self-scoped leader-side read/check-in surface. `GroupService.ListOperatorMembers` queries Better Auth's `member`/`"user"` tables directly (raw SQL via the sqlc stub, same pattern as the auth interceptor) to populate the leader picker — there's no separate "staff" concept, any Better Auth org member can be assigned as a group's leader. Until this existed, `pilgrims.group_id` (a real UUID FK) had no admin UI at all — `PilgrimFormDialog`'s old "Group code" field was a free-text input that would fail with a Postgres UUID-cast error the moment anyone typed a non-UUID label into it; it's now a `<select>` sourced from `GroupService.ListGroups`, the same pattern already used for `mahramId`.

**Module 5/6 (Group Leader + Pilgrim apps) are PWAs inside `apps/web`, not the native Expo scaffolds.** `apps/mobile-leader`/`apps/mobile-pilgrim` are still empty — there's no simulator/emulator here to verify a real native build, so both were built as mobile-web routes instead: `/pilgrim` (authenticated via Better Auth session — pilgrims sign in the same way as staff/leaders, matched by `pilgrims.email`, see the `databaseHooks.session.create.after` hook in `apps/web/lib/auth.ts`) and `/leader` (authenticated, self-scoped to `groups.leader_id = current user`). Neither route puts an identity UUID in the URL: `app_access_code`/`group_id` are resolved once from the session (`IdentityService.GetMyAccess`) and shared via `PilgrimCodeProvider`/`usePilgrimCode()` (`lib/pilgrim-context.tsx`) and `LeaderGroupProvider`/`useLeaderGroup()` (`lib/leader-context.tsx`) — a leader with more than one group gets a switcher instead of a route segment. `GroupLeaderService` in the backend never trusts a `group_id` from the request without calling `GroupLeaderRepository.EnsureLeaderOwnsGroup` first.

**`groups.leader_id` is `TEXT` referencing Better Auth's `"user"(id)`, not the `users` (plural) table** — that table was a vestigial pre-Better-Auth Clerk-era artifact (UUID-keyed, 0 rows, only referenced by this one FK) and was dropped in migration `025`. If you ever see code or a stale migration referencing plural `users`, it's wrong — Better Auth's real identity table is singular `"user"` (quoted, reserved word), joined the same raw-SQL way `internal/middleware/auth.go` already does.

**sqlc needs a schema stub for Better Auth's tables.** sqlc only models `db/migrations/` + `db/schema_stubs/better_auth.sql` (see `sqlc.yaml`) — the latter is a sqlc-only stand-in for `"user"` so queries can `JOIN` it; Better Auth migrates that table itself (`npx better-auth migrate`), goose never touches the stub. **A silently-wrong bug to watch for**: `db.XxxParams{...}` is a keyed struct literal — omitting a field (e.g. forgetting `Body:` on an INSERT) compiles fine and silently inserts the zero value instead of erroring. This actually happened (`ChatRepository.CreateFromUser` shipped without `Body` once) — when adding a new sqlc query, diff the params struct's field list against your literal before trusting it.

**Offline strategy is deliberately simple, not PowerSync.** Both PWAs use `apps/web/lib/offline.ts`: `cachedFetch` reads-through to `localStorage` and falls back to the last-cached value on failure, and `enqueueAction`/`useOfflineQueueFlush` queue write actions (SOS alerts, check-ins, chat sends) in `localStorage` and replay them on the browser's `online` event. Serwist compiles `app/sw.ts` into the generated/ignored `public/sw.js` during production builds, precaching every `/pilgrim` and `/leader` route plus its build assets for cold-start offline. `RequireAccess` keeps a bounded 72-hour access snapshot and the leader group list is read-through cached so the shell's network-only auth checks do not defeat that precache. This is still last-seen data plus queued writes, not PowerSync-grade conflict resolution or a device-verified 72-hour sync guarantee.

**Firebase push (SOS escalation) is two separate, independently-optional pieces**, both no-ops when unconfigured: backend (`FIREBASE_SERVICE_ACCOUNT_JSON` → `internal/notification.NewFirebasePusher`, called from `SOSService` on every alert and from the worker's `sos:escalate` sweep) and frontend (`NEXT_PUBLIC_FIREBASE_*` + `NEXT_PUBLIC_VAPID_PUBLIC_KEY` → `lib/firebase.ts`'s `requestPushToken`, registered via `NotificationService.RegisterPushSubscription`). In production Firebase Messaging is bundled into the same root-scope Serwist worker as offline caching; do not register a second production worker at `/`. The `/firebase-messaging-sw.js` route handler remains only as the development fallback because Serwist is disabled under Turbopack. It's in `middleware.ts`'s `PUBLIC_PATHS` and must remain fetchable unauthenticated for that fallback.

**Frontend → API.** `apps/web` (Next.js 15) never calls the Go API with raw `fetch`; it goes through Connect clients in `apps/web/lib/rpc.ts`, built on a transport (`apps/web/lib/transport.ts`) that attaches the Better Auth session token as a Bearer header on every request.

**Monorepo layout** (pnpm workspaces + turbo, packages under `apps/*` and `packages/*`):
- `apps/web` — Next.js operator dashboard + auth flows.
- `apps/api` — Go backend (net/http + Connect, pgx, sqlc, goose migrations).
- `apps/mobile-leader`, `apps/mobile-pilgrim` — Expo apps (currently minimal/scaffolded).
- `packages/proto-gen` — generated TS proto/Connect code, consumed by `apps/web` (and eventually mobile).
- `packages/ui` — shared design tokens (`tokens.ts`).
- `packages/validations` — shared validation logic (currently minimal).
- `proto/hajj/v1` — the single proto source of truth for both API surfaces.

**Database migrations** are plain numbered SQL files in `apps/api/db/migrations/`, applied with `goose`. sqlc reads those same migration files as schema plus `apps/api/db/query/*.sql` to generate `internal/gen/db`.

## Business rules worth knowing before touching related code

These live only in `CODEX_SPEC.md` section 7, not obviously in the code — check current service logic before assuming behavior, but they're the intended invariants:

- Room allocation: one room per pilgrim per hotel per season, capacity enforced, gender constraint (FAMILY = mahram pairs only), substitutions cascade all allocations in a single transaction.
- Seat assignment: one vehicle per pilgrim per movement; mahram pairs must share a vehicle.
- Orders/commissions: `platformMargin + operatorMargin + agentMargin ≤ 1.0`, validated on product save; `agentCommission = 0` when there's no `agentId`.
- Substitutions are irreversible once `isSubstituted = true`, and must always write an audit log entry.

**Error observability (Sentry).** Both apps report unhandled/internal errors to Sentry when a DSN is configured, and are silent no-ops when it isn't (safe for local dev). Backend: `SENTRY_DSN` in `apps/api/.env`, initialized in `cmd/server/main.go`, captured in `internal/service/errors.go`'s `serviceError` — only truly unmapped (`CodeInternal`) errors are reported; the client now gets a generic "internal error" message instead of the wrapped Go error (previously leaked raw error text, including DB errors, to callers). Frontend: `NEXT_PUBLIC_SENTRY_DSN` in `apps/web/.env.local`, wired via `apps/web/instrumentation.ts` (server/edge), `apps/web/instrumentation-client.ts` (browser), and `apps/web/app/global-error.tsx` (React render errors) — the standard `@sentry/nextjs` App Router layout, not the wizard-generated one.

## Coding conventions

- Go: `snake_case` filenames, `PascalCase` types. Never `panic` in request handlers; never expose raw DB errors to clients — map errors through the `AppError`/`connectError` pattern used in `internal/service/errors.go` and handlers.
- TypeScript: `strict: true`, no `any` — use generated types from `@hajj-saas/proto-gen` instead of hand-rolled interfaces for anything crossing the API boundary.
- TS files `kebab-case`, components `PascalCase`.
