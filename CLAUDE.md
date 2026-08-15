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

The Go API refuses to start unless `DATABASE_URL`, `BETTER_AUTH_SECRET`, and `CORS_ALLOWED_ORIGIN` are all set (see `apps/api/internal/config/config.go`). `cmd/worker` is a separate entrypoint requiring `DATABASE_URL` and `REDIS_URL` — it runs an asynq scheduler + server that recomputes agent tiers (`internal/worker/tier.go`) every 5 minutes. It does **not** compute payouts — see the `TODO(payout)` in `internal/service/agent.go`.

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

**Public (unauthenticated) RPCs are an explicit allowlist, not a default.** `internal/middleware/auth.go` wraps every Connect RPC with session validation by default; `publicProcedures` in that file is a deliberately small map of exceptions (currently only `AgentService/ApplyAsAgent`, used by the public `/apply/[operatorId]` page). Any handler reachable through that allowlist gets no identity from context — it must take `operator_id` from the request body and validate it itself, since there's no session to trust. Treat adding to this map as a security decision, not routing plumbing.

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

## Coding conventions

- Go: `snake_case` filenames, `PascalCase` types. Never `panic` in request handlers; never expose raw DB errors to clients — map errors through the `AppError`/`connectError` pattern used in `internal/service/errors.go` and handlers.
- TypeScript: `strict: true`, no `any` — use generated types from `@hajj-saas/proto-gen` instead of hand-rolled interfaces for anything crossing the API boundary.
- TS files `kebab-case`, components `PascalCase`.
