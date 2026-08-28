# Hajj & Umrah Operator SaaS — Codex Technical Specification

> **Purpose:** This document is the single source of truth for all AI-assisted development.
> Every Codex session must start by reading this file.
> Do not deviate from the stack, schema, or conventions defined here.

---

## 1. Product Overview

A B2B SaaS platform for licensed Hajj & Umrah tour operators to manage pilgrim groups end-to-end.
Replaces WhatsApp + Excel with a purpose-built system across three products:

| Product | Users | Platform |
|---|---|---|
| **Operator Dashboard** | Operations Manager, Coordinator | Web (Next.js) + PWA installable |
| **Group Leader App** | Muttawwif / Group Leader | **Built as a PWA** (`/leader`) — see the STATUS note below |
| **Pilgrim App** | Pilgrim | **Built as a PWA** (`/pilgrim/[code]`) — see the STATUS note below |

> **PWA strategy — revised from the original plan.** All three surfaces
> shipped as PWAs inside `apps/web`, not native. The original reasoning
> ("Group Leader App must remain native — PWA cannot support PowerSync
> offline sync or reliable iOS push") held only *if* PowerSync was the
> offline mechanism; since there was no simulator/device available in this
> environment to build and verify a real native app against anyway, both
> apps used the hand-rolled cache-and-queue in §11 instead — genuinely
> short-term (no 72h zero-network guarantee), not PowerSync-grade, but
> real and verified live. `apps/mobile-leader`/`apps/mobile-pilgrim`
> remain empty Expo scaffolds if native work resumes later.

### Revenue Streams
1. SaaS subscription per operator per season (SAR 500–3,000)
2. Commission on digital product sales (roaming/eSIM) to pilgrims (15–20% platform margin)
3. Referral agent commissions (SAR 200–500 per operator converted)

---

## 2. Tech Stack

> Use these exact versions. Do not substitute.
> Every choice below is optimised for: runtime performance, DX (easy to debug), and long-term maintainability.

### Core
```
Runtime (local):  Bun 1.x  (3x faster installs + local dev vs Node)
Runtime (prod):   Node.js 20 LTS  (Docker container)
Language:         TypeScript 5.x  (strict: true everywhere — non-negotiable)
Package manager:  pnpm 9.x
Monorepo:         Turborepo 2.x
```

### Web App (Operator Dashboard + Pilgrim PWA)
```
Framework:        Next.js 15 (App Router + React 19)
UI:               Tailwind CSS 4.x + shadcn/ui
Auth (web):       better-auth + better-auth/react
                  - Email/password + organization (multi-tenant) plugin
                  - All auth data stored in your own PostgreSQL
                  - Session cookie-based — no external dependency
API client:       @connectrpc/connect-web + @connectrpc/connect-query
Server state:     TanStack Query v5  (via connect-query — one hook per RPC)
Client state:     Zustand 5.x
Forms:            React Hook Form + Zod
Tables:           TanStack Table v8
Charts:           Recharts
PDF export:       @react-pdf/renderer  (background job — not inline)
Excel export:     xlsx (SheetJS)
PWA:              Serwist-generated `public/sw.js` + `lib/offline.ts` (see §11)
                  - Pilgrim PWA (/pilgrim/[code]) AND Group Leader PWA (/leader) — both, not just pilgrim
                  - route/build precache plus `cachedFetch`/`enqueueAction` cache-and-queue
                  - Firebase Cloud Messaging for coordinator/leader push (independently optional)
```

### Backend API (Go — gRPC via Connect)
```
Language:         Go 1.22+
RPC Protocol:     Connect (by Buf)
                  - gRPC-compatible, works natively in browsers + React Native
                  - No Envoy proxy needed (unlike raw gRPC)
                  - Supports HTTP/1.1 and HTTP/2
                  - Server-side streaming for SOS alerts and chat
                  - Library: connectrpc.com/connect
HTTP base:        net/http (standard library — Connect wraps it)
                  + golang.org/x/net/http2 for HTTP/2 support
Proto toolchain:  Buf CLI
                  - buf.yaml — defines module and lint rules
                  - buf.gen.yaml — code generation config
                  - buf generate → produces Go server stubs + TypeScript client
                  - buf breaking → auto-detect breaking changes before deploy
Query layer:      sqlc  (SQL → type-safe Go code)
                  - Write raw .sql files in db/query/
                  - sqlc generates typed Go functions — zero ORM overhead
                  - Every query is readable, auditable, testable
DB Driver:        pgx/v5  (fastest PostgreSQL driver for Go)
Migrations:       goose  (SQL-based, version-controlled)
Database:         PostgreSQL 16  (self-hosted, Docker Compose on VPS)
                  - Managed via docker-compose.yml alongside the API container
                  - Single DATABASE_URL — no pooler bypass needed (pgx manages pool)
                  - Persistent volume: /var/lib/postgresql/data on VPS
Auth:             Better Auth — opaque DB session token, NOT a JWT (see §5)
                  - Better Auth (running in Next.js) issues an opaque token, stored in its own `session` table
                  - Go looks the token up directly via a DB join into `session`/`member`/`"user"` — no signature to validate, no shared-secret verification library needed
                  - Connect interceptor derives operatorId + userId from the DB row, in three trust lanes (default/sessionOnly/public — see §5), not from decoded claims
                  - No external auth service — all data in your own PostgreSQL
                  - Google Sign-In added as an additional `socialProviders` provider — still the same DB-session mechanism, not OAuth-issued JWTs either
Validation:       protovalidate-go  (validate proto messages in Go — no separate validator)
Cache + Queue:    Redis — **self-hosted via Docker Compose** (see DEPLOY.md §4), not Upstash
                  - asynq job queue broker only (agent tier recalc + SOS escalation, `cmd/worker`)
                  - The app's own rate limiting is in-memory per-process (`internal/middleware/ratelimit.go`), not Redis-backed
Background jobs:  asynq  (Redis-backed, Go-native)
                  - Goroutine pool in same binary
                  - Real periodic tasks: agent tier recalc every 5min, SOS escalation every 1min — no asynqmon UI built
Push notif:       Firebase Admin Go SDK — optional, no-op when `FIREBASE_SERVICE_ACCOUNT_JSON` is unset
Email:            Hostinger SMTP, via `apps/web/lib/email.ts` (Nodemailer) — password reset, email verification, invitations, and 2FA enrolment OTP. No-op (logged) when SMTP credentials are unset.
WhatsApp/SMS:     ~~Twilio REST API~~ — never built, not in current `.env.example`
File storage:     ~~Cloudflare R2~~ — never built, no code path reads `R2_*` vars yet; reserved for future product-image/PDF-export storage
Observability:    Sentry Go SDK (`SENTRY_DSN`, optional/no-op when unset) + structured JSON logging via `slog` — Axiom vars exist in `.env.example` but log shipping isn't wired up
```

### Web + Mobile API Communication
```
Protocol:         Connect (gRPC-compatible, runs in browser natively)
TypeScript client: @connectrpc/connect
React Query:      @connectrpc/connect-query  (TanStack Query integration)
Proto types:      @bufbuild/protobuf  (generated by buf from proto files)

Type safety flow:
  proto/hajj/v1/pilgrim.proto
    → buf generate
    → Go: internal/gen/hajj/v1/pilgrim_grpc.go + pilgrim.pb.go
    → TypeScript: packages/proto-gen/hajj/v1/pilgrim_pb.ts
    → Use in web: import { Pilgrim } from '@hajj-saas/proto-gen/hajj/v1/pilgrim_pb'
    → Use in mobile: same import
```

### Mobile Apps (Group Leader App + Pilgrim App)

> **STATUS: superseded for the MVP.** `apps/mobile-leader`/`apps/mobile-pilgrim`
> below are still empty Expo scaffolds — there was no simulator/emulator/device
> available to verify a real native build against, so both were built instead
> as mobile-web PWAs inside `apps/web` (`/leader`, `/pilgrim/[code]`), using
> the hand-rolled offline cache in §11, not PowerSync. This section is kept as
> the original native-app intent in case that work resumes later; treat §11
> (PWA Architecture) as the actual current implementation.

```
Framework:        Expo SDK 52 + React Native 0.76
Styling:          NativeWind 4.x
Navigation:       Expo Router v4 (file-based)
Offline sync:     PowerSync
                  - Designed specifically for PostgreSQL → mobile sync
                  - Works with any backend (including Go) via HTTP connector
                  - Handles conflicts automatically
State:            Zustand 5.x
API client:       @connectrpc/connect + @connectrpc/connect-query
Push notif:       Expo Notifications + Firebase Cloud Messaging
QR scan:          expo-camera
Secure storage:   expo-secure-store  (NEVER AsyncStorage for tokens/keys)
```

### Observability (non-negotiable from day 1)
```
Error tracking:   Sentry  (Web Next.js SDK + Mobile Expo SDK)
Logging:          Axiom   (structured JSON logs, queryable)
Analytics:        PostHog (feature flags, product analytics, A/B testing)
```

### Infrastructure
```
Hosting (web):    VPS  (Next.js runs in Docker container)
Hosting (API):    VPS  (Go binary in Docker container)
Hosting (DB):     VPS  (PostgreSQL 16 in Docker Compose — same server)
Reverse proxy:    nginx  (terminates SSL, routes /api → Go port 9100, / → Next.js port 9101)
SSL:              Let's Encrypt via Certbot (auto-renew)
File storage:     Cloudflare R2
CDN:              Cloudflare (auto via R2)
CI/CD:            GitHub Actions  (test → build → push GHCR → SSH deploy)
Container:        Docker Compose  (api + web + postgres)
Ports (host):     127.0.0.1:9100 (API), 127.0.0.1:9101 (Web) — never exposed to internet
```

### Why each key decision was made

| Decision | Choice | Why |
|---|---|---|
| Backend language | **Go + net/http + Connect** | Connect produces a `net/http.Handler`; Fiber is incompatible and was never used. |
| Query layer | **sqlc** | Write SQL, get typed Go — no ORM magic, no hidden queries |
| DB | **PostgreSQL 16 self-hosted** | Full control, zero hosting cost, no vendor dependency |
| Hosting | **VPS + Docker Compose** | Own infrastructure, predictable cost, nginx handles SSL + routing |
| Auth | **Better Auth** | Open source, multi-tenant orgs built-in, ALL data in your own PostgreSQL — zero external dependency |
| Offline sync | ~~PowerSync~~ → **hand-rolled cache** | No device/simulator available to verify a real native+PowerSync build; short-term `localStorage` cache-and-queue instead (§11), not a 72h guarantee |
| Type safety | **Protocol Buffers + Connect** | Single `.proto` → type-safe Go server + TS client. No tRPC (TS-only), no OpenAPI hand-sync |
| Background jobs | **asynq + Redis** | Go-native, same binary, self-hosted Redis via Docker Compose (not Upstash — see DEPLOY.md §4) |
| Old: Prisma | **sqlc + pgx** | Prisma can't run in Go |
| Old: Inngest | **asynq** | Go-native, no external service |
| Old: Clerk | **Better Auth** | Clerk stores your user data on their servers. Better Auth = full ownership |

---

## 3. Monorepo Structure

```
hajj-saas/
├── apps/
│   ├── api/                        # Go backend (standalone Go module)
│   │   ├── cmd/server/main.go      # entry point — starts net/http + asynq worker
│   │   ├── internal/
│   │   │   ├── handler/            # Connect RPC handlers (one file per domain)
│   │   │   ├── service/            # business logic (no HTTP, no DB)
│   │   │   ├── repository/         # DB access (wraps sqlc-generated code)
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go         # Better Auth JWT validation
│   │   │   │   ├── tenant.go       # extract operatorId from JWT
│   │   │   │   ├── ratelimit.go
│   │   │   │   └── sentry.go
│   │   │   └── worker/             # asynq background jobs
│   │   ├── db/
│   │   │   ├── query/              # .sql files (input to sqlc)
│   │   │   ├── migrations/         # goose SQL migrations
│   │   │   └── generated/          # sqlc output — DO NOT EDIT
│   │   ├── go.mod
│   │   └── sqlc.yaml
│   │
│   ├── web/                        # Next.js — Operator Dashboard + Pilgrim PWA
│   │   ├── app/
│   │   │   ├── api/auth/[...all]/route.ts  # Better Auth handler
│   │   │   ├── (auth)/             # sign-in, sign-up pages
│   │   │   ├── (dashboard)/        # Operator Dashboard (session-protected)
│   │   │   └── pilgrim/[code]/     # Pilgrim PWA (auth via appAccessCode only)
│   │   ├── lib/
│   │   │   ├── auth.ts             # Better Auth server instance
│   │   │   ├── auth-client.ts      # Better Auth React client
│   │   │   ├── transport.ts        # Connect transport
│   │   │   └── rpc.ts              # typed service clients
│   │   └── public/
│   │       ├── manifest.json       # PWA manifest
│   │       └── icons/
│   │
│   ├── mobile-leader/              # Expo — Group Leader App
│   └── mobile-pilgrim/             # Expo — Pilgrim App
│
├── proto/                          # SINGLE SOURCE OF TRUTH for all API contracts
│   ├── buf.yaml
│   ├── buf.gen.yaml
│   └── hajj/v1/
│       ├── pilgrim.proto
│       ├── group.proto
│       ├── accommodation.proto
│       ├── transport.proto
│       ├── product.proto
│       ├── order.proto
│       ├── agent.proto
│       ├── sos.proto
│       ├── chat.proto
│       ├── sync.proto
│       └── common.proto
│
├── packages/
│   ├── proto-gen/                  # Auto-generated by buf — DO NOT EDIT
│   ├── validations/                # Zod schemas for form validation
│   └── ui/                         # Shared design tokens
│
├── deploy/
│   ├── docker-compose.prod.yml
│   └── nginx/safrat.conf
│
└── .github/workflows/deploy.yml
```

---

## 4. Database Schema (PostgreSQL — goose migrations + sqlc)

> **Files:** `apps/api/db/migrations/*.sql` (goose), `apps/api/db/query/*.sql` (sqlc input)
> Never use Prisma. Go backend uses sqlc + pgx/v5 directly.
> Run migrations: `goose -dir db/migrations postgres $DATABASE_URL up`
> Connection: single `DATABASE_URL` — pgx manages its own pool. No `DIRECT_URL` needed.

> **Better Auth tables** are created separately via `npx better-auth migrate` in apps/web.
> They live in the same PostgreSQL database alongside the business schema.

```sql
-- ─── OPERATOR (multi-tenant root) ───────────────────────────────────────────
-- Note: betterAuthUserId links to Better Auth's user table (created by better-auth migrate)

CREATE TABLE operators (
  id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  better_auth_org_id TEXT UNIQUE NOT NULL,  -- Better Auth organization ID
  name            TEXT NOT NULL,
  license_number  TEXT,
  country         TEXT NOT NULL DEFAULT '',
  phone           TEXT,
  email           TEXT NOT NULL,
  logo_url        TEXT,
  plan            TEXT NOT NULL DEFAULT 'STARTER', -- STARTER | GROWTH | PRO
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
  id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  better_auth_user_id TEXT UNIQUE NOT NULL,  -- Better Auth user ID
  operator_id     TEXT NOT NULL REFERENCES operators(id),
  role            TEXT NOT NULL,  -- MANAGER | COORDINATOR | GROUP_LEADER
  name            TEXT NOT NULL,
  email           TEXT NOT NULL,
  phone           TEXT,
  avatar_url      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── SEASON ──────────────────────────────────────────────────────────────────

CREATE TABLE seasons (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  operator_id TEXT NOT NULL REFERENCES operators(id),
  name        TEXT NOT NULL,
  type        TEXT NOT NULL,  -- HAJJ | UMRAH
  start_date  TIMESTAMPTZ NOT NULL,
  end_date    TIMESTAMPTZ NOT NULL,
  is_active   BOOLEAN NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── PILGRIM ─────────────────────────────────────────────────────────────────

CREATE TABLE pilgrims (
  id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  season_id           TEXT NOT NULL REFERENCES seasons(id),
  operator_id         TEXT NOT NULL REFERENCES operators(id),
  group_id            TEXT REFERENCES groups(id),
  full_name           TEXT NOT NULL,
  passport_number     TEXT NOT NULL,
  nationality         TEXT NOT NULL,
  date_of_birth       TIMESTAMPTZ NOT NULL,
  gender              TEXT NOT NULL,  -- MALE | FEMALE
  photo_url           TEXT,
  phone               TEXT,
  emergency_contact   TEXT,
  preferred_lang      TEXT NOT NULL DEFAULT 'ar',
  medical_notes       TEXT,
  requires_wheelchair BOOLEAN NOT NULL DEFAULT false,
  mahram_id           TEXT REFERENCES pilgrims(id),
  is_substituted      BOOLEAN NOT NULL DEFAULT false,
  substituted_by_id   TEXT,
  app_access_code     TEXT UNIQUE NOT NULL DEFAULT gen_random_uuid()::text,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(season_id, passport_number)
);

-- ─── GROUP ───────────────────────────────────────────────────────────────────

CREATE TABLE groups (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  season_id   TEXT NOT NULL REFERENCES seasons(id),
  operator_id TEXT NOT NULL REFERENCES operators(id),
  leader_id   TEXT REFERENCES users(id),
  name        TEXT NOT NULL,
  capacity    INT NOT NULL DEFAULT 40,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── ACCOMMODATION ───────────────────────────────────────────────────────────

CREATE TABLE hotels (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  season_id     TEXT NOT NULL REFERENCES seasons(id),
  operator_id   TEXT NOT NULL REFERENCES operators(id),
  name          TEXT NOT NULL,
  city          TEXT NOT NULL,  -- MAKKAH | MADINAH | MINA | ARAFAT
  address       TEXT,
  star_rating   INT,
  check_in_date  TIMESTAMPTZ NOT NULL,
  check_out_date TIMESTAMPTZ NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE rooms (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  hotel_id    TEXT NOT NULL REFERENCES hotels(id),
  room_number TEXT NOT NULL,
  floor       INT,
  building    TEXT,
  type        TEXT NOT NULL,    -- SINGLE | DOUBLE | TRIPLE | QUAD
  gender      TEXT NOT NULL,    -- MALE | FEMALE | FAMILY
  capacity    INT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(hotel_id, room_number)
);

CREATE TABLE room_allocations (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  room_id     TEXT NOT NULL REFERENCES rooms(id),
  pilgrim_id  TEXT NOT NULL REFERENCES pilgrims(id),
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  assigned_by TEXT NOT NULL,
  UNIQUE(pilgrim_id, room_id)
);

-- ─── TRANSPORT ───────────────────────────────────────────────────────────────

CREATE TABLE movements (
  id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  season_id    TEXT NOT NULL REFERENCES seasons(id),
  operator_id  TEXT NOT NULL REFERENCES operators(id),
  name         TEXT NOT NULL,
  origin       TEXT NOT NULL,
  destination  TEXT NOT NULL,
  scheduled_at TIMESTAMPTZ NOT NULL,
  status       TEXT NOT NULL DEFAULT 'SCHEDULED',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE vehicles (
  id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  movement_id  TEXT NOT NULL REFERENCES movements(id),
  plate_number TEXT NOT NULL,
  capacity     INT NOT NULL,
  driver_name  TEXT,
  driver_phone TEXT,
  status       TEXT NOT NULL DEFAULT 'SCHEDULED',
  departed_at  TIMESTAMPTZ,
  arrived_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE seat_assignments (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  vehicle_id  TEXT NOT NULL REFERENCES vehicles(id),
  pilgrim_id  TEXT NOT NULL REFERENCES pilgrims(id),
  seat_number INT,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(vehicle_id, pilgrim_id)
);

-- ─── DIGITAL PRODUCTS ────────────────────────────────────────────────────────

CREATE TABLE products (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  operator_id      TEXT NOT NULL REFERENCES operators(id),
  name             TEXT NOT NULL,
  description      TEXT,
  data_gb          INT,
  validity_days    INT,
  price_sar        DECIMAL(10,2) NOT NULL,
  third_party_code TEXT,
  is_active        BOOLEAN NOT NULL DEFAULT true,
  platform_margin  DECIMAL(5,4) NOT NULL DEFAULT 0.15,
  operator_margin  DECIMAL(5,4) NOT NULL DEFAULT 0.10,
  agent_margin     DECIMAL(5,4) NOT NULL DEFAULT 0.05,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE orders (
  id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  product_id          TEXT NOT NULL REFERENCES products(id),
  pilgrim_id          TEXT NOT NULL REFERENCES pilgrims(id),
  agent_id            TEXT REFERENCES agents(id),
  amount_sar          DECIMAL(10,2) NOT NULL,
  platform_commission DECIMAL(10,2) NOT NULL,
  operator_commission DECIMAL(10,2) NOT NULL,
  agent_commission    DECIMAL(10,2) NOT NULL DEFAULT 0,
  status              TEXT NOT NULL DEFAULT 'PENDING',
  third_party_ref     TEXT,
  activated_at        TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── AGENT / REFERRAL ────────────────────────────────────────────────────────

CREATE TABLE agents (
  id             TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  operator_id    TEXT REFERENCES operators(id),
  name           TEXT NOT NULL,
  email          TEXT UNIQUE NOT NULL,
  phone          TEXT,
  referral_code  TEXT UNIQUE NOT NULL DEFAULT gen_random_uuid()::text,
  tier           TEXT NOT NULL DEFAULT 'STANDARD',
  total_earnings DECIMAL(10,2) NOT NULL DEFAULT 0,
  pending_payout DECIMAL(10,2) NOT NULL DEFAULT 0,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE referrals (
  id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  agent_id        TEXT NOT NULL REFERENCES agents(id),
  referred_email  TEXT NOT NULL,
  operator_id     TEXT REFERENCES operators(id),
  status          TEXT NOT NULL DEFAULT 'PENDING',
  commission_sar  DECIMAL(10,2) NOT NULL DEFAULT 0,
  paid_at         TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE commissions (
  id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  agent_id   TEXT NOT NULL REFERENCES agents(id),
  type       TEXT NOT NULL,  -- REFERRAL | PRODUCT_SALE
  amount_sar DECIMAL(10,2) NOT NULL,
  source_id  TEXT NOT NULL,
  paid_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── PILGRIM APP FEATURES ────────────────────────────────────────────────────

CREATE TABLE sos_alerts (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  pilgrim_id       TEXT NOT NULL REFERENCES pilgrims(id),
  operator_id      TEXT NOT NULL REFERENCES operators(id),
  latitude         DOUBLE PRECISION,
  longitude        DOUBLE PRECISION,
  location_note    TEXT,
  status           TEXT NOT NULL DEFAULT 'TRIGGERED',
  acknowledged_at  TIMESTAMPTZ,
  acknowledged_by  TEXT,
  resolved_at      TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE check_ins (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  pilgrim_id    TEXT NOT NULL REFERENCES pilgrims(id),
  movement_id   TEXT NOT NULL REFERENCES movements(id),
  type          TEXT NOT NULL,    -- DEPARTURE | ARRIVAL
  method        TEXT NOT NULL,    -- SELF_QR | MANUAL
  checked_by_id TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_messages (
  id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  group_id     TEXT NOT NULL REFERENCES groups(id),
  sender_id    TEXT NOT NULL,
  sender_type  TEXT NOT NULL,  -- PILGRIM | GROUP_LEADER | COORDINATOR
  content      TEXT NOT NULL,
  is_read      BOOLEAN NOT NULL DEFAULT false,
  sent_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  delivered_at TIMESTAMPTZ
);

CREATE TABLE audit_logs (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  operator_id TEXT NOT NULL,
  user_id     TEXT NOT NULL,
  action      TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id   TEXT NOT NULL,
  metadata    JSONB,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 5. API Design (Protocol Buffers — Connect/gRPC)

> All API contracts are defined in `proto/hajj/v1/*.proto`.
> Run `buf generate proto/` after any proto change.
> Run `buf breaking proto/ --against '.git#branch=main'` before any PR.

### buf.gen.yaml
```yaml
version: v2
plugins:
  - remote: buf.build/connectrpc/go
    out: apps/api/internal/gen
    opt: paths=source_relative
  - remote: buf.build/bufbuild/go
    out: apps/api/internal/gen
    opt: paths=source_relative
  - remote: buf.build/es/protobuf-es
    out: packages/proto-gen
    opt: target=ts
  - remote: buf.build/connectrpc/es
    out: packages/proto-gen
    opt: target=ts
```

### Authentication (Connect interceptor — Better Auth DB session, not JWT)

> **Correction vs. the original plan:** Better Auth's default session
> strategy issues an **opaque, database-backed token**, not a JWT — there
> are no claims to decode locally, so the interceptor below looks the
> session up by joining straight into Better Auth's own `session`/`member`/
> `"user"` tables on every request instead of verifying a signature. Real
> implementation is `apps/api/internal/middleware/auth.go`; this is a
> summary, not the literal file.

```go
// Three trust lanes, not one:
//
// 1. Default (most RPCs) — requires a valid session AND organization
//    membership. operatorID and userID come from the DB row, never from
//    the request body.
// 2. sessionOnlyProcedures — requires a valid session but explicitly NOT
//    organization membership (e.g. IdentityService.GetMyAccess,
//    PilgrimAppService.LinkGoogleAccount). A pilgrim who links a Google
//    account is a real Better Auth user but never an org member.
// 3. publicProcedures — no session at all; the pilgrim/agent-application
//    surface, authenticated instead by an unguessable UUID
//    (app_access_code) carried in the request body. Every addition to
//    this list is a deliberate security decision, not routing plumbing.
func NewAuthInterceptor(pool *pgxpool.Pool) connect.Interceptor {
    return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            if publicProcedures[req.Spec().Procedure] {
                return next(ctx, req) // identity comes from the request body instead
            }
            token, err := bearerToken(req.Header().Get("Authorization"))
            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, err)
            }
            if sessionOnlyProcedures[req.Spec().Procedure] {
                // session required, org membership NOT required
                userID, userEmail, err := lookupSessionOnly(ctx, pool, token)
                // ... inject userID/userEmail into ctx, call next(ctx, req)
            }
            // default lane: session AND org membership required
            userID, operatorID, userName, err := lookupSessionWithOrg(ctx, pool, token)
            // ... inject into ctx, call next(ctx, req)
        }
    })
}

// operatorID always comes from the verified session — never trust a
// client-provided operatorId in the request body.
func OperatorIDFromCtx(ctx context.Context) string { /* ... */ }
func UserIDFromCtx(ctx context.Context) string     { /* ... */ }
func UserEmailFromCtx(ctx context.Context) string  { /* ... */ } // sessionOnly lane only
```

### RBAC / single shared login (`IdentityService`)

Operator staff (Better Auth org members), Group Leaders/Muttawwif
(`groups.leader_id`), and Pilgrims (`pilgrims.linked_user_id`, set once a
pilgrim optionally links a Google account from their `/pilgrim/[code]`
link) all sign in through the **same** `/sign-in` page on one domain —
there's no separate login per surface. `IdentityService.GetMyAccess`
(session-only lane, see above) resolves which of those three role systems
the calling identity participates in — an identity can be more than one
at once — and the frontend's `resolveLandingPath()` picks where to land by
Better Auth's own org-role tier, not a flat "is an org member" boolean:

1. `owner`/`admin` org role → `/dashboard` (actual administrators)
2. leads ≥1 group → `/leader` (their real job — a `member`-role Muttawwif
   must still be an org member to be assignable from the admin Groups
   page, but that's an implementation detail, not their function)
3. `member`-role org member with no group → `/dashboard` (fallback)
4. linked pilgrim → `/pilgrim/[code]`
5. no role yet → `/onboarding` ("create your operator")

`/dashboard` and `/leader` each mount a `RequireAccess` guard (real check
against `GetMyAccess`, TTL-cached both server- and client-side — see
`internal/repository/identity.go` and `lib/access-cache.ts`) — `middleware.ts`
alone only proves a session *cookie* is present, not that the identity
behind it belongs on that surface, so the guard is what actually enforces
this, not routing convention.

Sessions are **single-session, enforced server-side**: signing in anywhere
immediately revokes every other active session for that user
(`databaseHooks.session.create.after` in `lib/auth.ts`), no grace period —
deliberate, since money will eventually flow through this login
(Module 7).

### Proto Service Definitions

```proto
// proto/hajj/v1/pilgrim.proto
syntax = "proto3";
package hajj.v1;
import "google/protobuf/timestamp.proto";
import "buf/validate/validate.proto";

service PilgrimService {
  rpc CreatePilgrim    (CreatePilgrimRequest)    returns (Pilgrim);
  rpc ListPilgrims     (ListPilgrimsRequest)     returns (ListPilgrimsResponse);
  rpc GetPilgrim       (GetPilgrimRequest)       returns (Pilgrim);
  rpc UpdatePilgrim    (UpdatePilgrimRequest)    returns (Pilgrim);
  rpc DeletePilgrim    (DeletePilgrimRequest)    returns (DeletePilgrimResponse);
  rpc ImportPilgrims   (ImportPilgrimsRequest)   returns (ImportPilgrimsResponse);
  rpc SubstitutePilgrim(SubstituteRequest)       returns (SubstituteResponse);
  rpc GetAllocations   (GetAllocationsRequest)   returns (PilgrimAllocations);
}
```

```proto
// proto/hajj/v1/sos.proto
service SosService {
  rpc TriggerSos      (TriggerSosRequest)   returns (SosAlert);
  rpc AcknowledgeSos  (AcknowledgeRequest)  returns (SosAlert);
  rpc ResolveSos      (ResolveRequest)      returns (SosAlert);
  rpc StreamSosAlerts (StreamSosRequest)    returns (stream SosAlert);
}
```

```proto
// proto/hajj/v1/chat.proto
service ChatService {
  rpc SendMessage    (SendMessageRequest)   returns (ChatMessage);
  rpc ListMessages   (ListMessagesRequest)  returns (ListMessagesResponse);
  rpc StreamMessages (StreamRequest)        returns (stream ChatMessage);
}
```

```proto
// proto/hajj/v1/accommodation.proto
service AccommodationService {
  rpc CreateHotel      (CreateHotelRequest)  returns (Hotel);
  rpc ListHotels       (ListHotelsRequest)   returns (ListHotelsResponse);
  rpc CreateRoom       (CreateRoomRequest)   returns (Room);
  rpc GetRoomsByHotel  (GetRoomsRequest)     returns (GetRoomsResponse);
  rpc AllocateRoom     (AllocateRoomRequest) returns (RoomAllocation);
  rpc DeallocateRoom   (DeallocateRequest)   returns (DeallocateResponse);
  rpc GetHotelManifest (ManifestRequest)     returns (HotelManifest);
}
```

```proto
// proto/hajj/v1/transport.proto
service TransportService {
  rpc CreateMovement      (CreateMovementRequest) returns (Movement);
  rpc ListMovements       (ListMovementsRequest)  returns (ListMovementsResponse);
  rpc AddVehicle          (AddVehicleRequest)     returns (Vehicle);
  rpc AssignSeat          (AssignSeatRequest)     returns (SeatAssignment);
  rpc MarkDeparted        (DepartRequest)         returns (Vehicle);
  rpc MarkArrived         (ArriveRequest)         returns (Vehicle);
  rpc GetMovementManifest (ManifestRequest)       returns (MovementManifest);
}
```

```proto
// proto/hajj/v1/pilgrim_app.proto — public (auth via appAccessCode)
service PilgrimAppService {
  rpc GetMyInfo      (GetMyInfoRequest)  returns (PilgrimAppInfo);
  rpc SelfCheckIn    (CheckInRequest)   returns (CheckIn);
  rpc StreamSchedule (ScheduleRequest)  returns (stream ScheduleUpdate);
}
```

---

## 6. Feature Modules — Build Order

Build strictly in this order. Each module depends on the previous.

---

### MODULE 0: Monorepo Bootstrap

**Buf / proto setup:**
```bash
brew install bufbuild/buf/buf
mkdir -p proto/hajj/v1
buf generate proto/
```

**Go backend setup:**
```bash
cd apps/api
go mod init github.com/your-org/hajj-saas/api
go get connectrpc.com/connect
go get golang.org/x/net/http2
go get github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go get github.com/pressly/goose/v3/cmd/goose@latest
go get github.com/jackc/pgx/v5
go get github.com/hibiken/asynq
go get github.com/golang-jwt/jwt/v5
go get github.com/bufbuild/protovalidate-go
go get github.com/getsentry/sentry-go

# Run first migration
goose -dir db/migrations postgres $DATABASE_URL up
```

**Local PostgreSQL (Docker Compose — already running):**
```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: safrat
      POSTGRES_PASSWORD: password
      POSTGRES_DB: safrat
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
```

**Better Auth setup (web):**
```bash
cd apps/web
pnpm add better-auth
```

```typescript
// apps/web/lib/auth.ts
import { betterAuth } from "better-auth"
import { organization } from "better-auth/plugins"
import { Pool } from "pg"

export const auth = betterAuth({
  database: new Pool({ connectionString: process.env.DATABASE_URL }),
  plugins: [
    organization({
      allowUserToCreateOrganization: true,
    }),
  ],
  emailAndPassword: { enabled: true },
  session: { expiresIn: 60 * 60 * 24 * 7 }, // 7 days
  secret: process.env.BETTER_AUTH_SECRET!,
})
```

```typescript
// apps/web/app/api/auth/[...all]/route.ts
import { auth } from "@/lib/auth"
import { toNextJsHandler } from "better-auth/next-js"
export const { GET, POST } = toNextJsHandler(auth.handler)
```

```typescript
// apps/web/lib/auth-client.ts
import { createAuthClient } from "better-auth/react"
import { organizationClient } from "better-auth/client/plugins"

export const authClient = createAuthClient({
  baseURL: process.env.NEXT_PUBLIC_APP_URL!,
  plugins: [organizationClient()],
})
```

```bash
# Run Better Auth migrations (creates users, sessions, orgs tables in your PostgreSQL)
cd apps/web
npx better-auth migrate
```

**Middleware (session protection):**
```typescript
// apps/web/middleware.ts
import { auth } from "@/lib/auth"
import { NextRequest, NextResponse } from "next/server"

export async function middleware(request: NextRequest) {
  const session = await auth.api.getSession({ headers: request.headers })
  if (!session && request.nextUrl.pathname.startsWith("/dashboard")) {
    return NextResponse.redirect(new URL("/sign-in", request.url))
  }
  if (!session && request.nextUrl.pathname.startsWith("/onboarding")) {
    return NextResponse.redirect(new URL("/sign-in", request.url))
  }
  return NextResponse.next()
}

export const config = {
  matcher: ["/dashboard/:path*", "/onboarding/:path*"],
}
```

**Done when:** `pnpm dev` runs web + API. `go run ./cmd/server` starts API. `buf generate` produces clean output. Better Auth sign-in works on `/sign-in`.

---

### MODULE 1: Operator Onboarding

**Flow (no webhooks — fully synchronous):**
1. User signs up → Better Auth creates user + session
2. Onboarding page creates organization via Better Auth
3. Same request calls Go API `CreateOperator` RPC → inserts operator row linked to Better Auth org ID
4. Create first season → redirect to dashboard

```typescript
// apps/web/app/onboarding/page.tsx  — key flow
const { data: session } = authClient.useSession()

async function handleOnboardingComplete(data: OnboardingForm) {
  // 1. Create org in Better Auth
  const org = await authClient.organization.create({ name: data.companyName })

  // 2. Create operator in Go API (single source of truth for business data)
  await operatorClient.createOperator({
    betterAuthOrgId: org.id,
    name: data.companyName,
    country: data.country,
    email: session.user.email,
    licenseNumber: data.licenseNumber,
  })

  // 3. Create first season
  await seasonClient.createSeason({ name: data.firstSeasonName, type: data.seasonType, ... })

  router.push("/dashboard")
}
```

**Go API — CreateOperator RPC:**
- Handler: parse request, call service
- Service: validate betterAuthOrgId is not already registered (idempotency check)
- Repository: `INSERT INTO operators ... ON CONFLICT (better_auth_org_id) DO NOTHING RETURNING *`

**Key files:**
- `apps/api/internal/handler/operator.go`
- `apps/api/internal/service/operator.go`
- `apps/api/internal/repository/operator.go`
- `apps/web/app/sign-in/page.tsx` — Better Auth sign in form (no `(auth)` route group in the actual tree)
- `apps/web/app/sign-up/page.tsx` — Better Auth sign up form
- `apps/web/app/onboarding/page.tsx`

Both sign-in/sign-up also render a "Lanjutkan dengan Google" button
(`components/auth/GoogleSignInButton.tsx`) — additive, email/password
keeps working. See the RBAC section in §5 for how a signed-in identity
gets routed after either flow.

**Done when:** Operator can sign up, create org, create season, see dashboard.

---

### MODULE 2: Pilgrim Management

1. Pilgrim list with search + filter (group, **kloter**, gender, nationality, medical flag), pagination
2. Create pilgrim form — all fields, mahram selector, kloter selector
3. Bulk CSV import — column mapping wizard, row validation, batch insert
4. Edit pilgrim
5. Substitution — cascade transaction (room + seat + group)
6. Pilgrim detail page

**Kloter (added, not in the original plan):** the real Kemenag-level
departure batch — a numbered batch (e.g. "SOC-01") tied to one flight and
one departure date, distinct from **Rombongan/Group** (the pastoral
subdivision a Ketua Rombongan leads; one kloter typically contains several
rombongan). `KloterService` CRUD, `kloters` table
(`operator_id`/`season_id`/`code`/`embarkation`/`flight_number`/
`departure_date`/`capacity`), `pilgrims.kloter_id` nullable FK. Drives the
Operator Dashboard's per-kloter/per-season filters and scopes what a
pilgrim's PWA schedule shows (see Module 6).

**Substitution logic (Go service layer):**
```go
func (s *SubstitutionService) Execute(ctx context.Context, originalID, replacementID string) error {
    return s.db.BeginTxFunc(ctx, pgx.TxOptions{}, func(tx pgx.Tx) error {
        qtx := s.queries.WithTx(tx)
        if err := qtx.CopyRoomAllocations(ctx, ...); err != nil { return err }
        if err := qtx.CopySeatAssignments(ctx, ...); err != nil { return err }
        if err := qtx.CopyGroupAssignment(ctx, ...); err != nil { return err }
        if err := qtx.MarkSubstituted(ctx, ...); err != nil { return err }
        return qtx.CreateAuditLog(ctx, db.CreateAuditLogParams{
            OperatorID: OperatorIDFromCtx(ctx),
            UserID:     UserIDFromCtx(ctx),
            Action:     "PILGRIM_SUBSTITUTION",
            EntityType: "Pilgrim", EntityID: originalID,
        })
    })
}
```

**Done when:** 200 pilgrims importable in under 5 minutes. Substitution cascades correctly.

---

### MODULE 3: Accommodation Allocator

1. Hotel CRUD (name, city, star rating, check-in/out)
2. Room setup (number, floor, building, type, gender, capacity)
3. Allocation UI: drag pilgrim → room, or select + button
4. Validation: gender conflict, over-capacity, duplicate
5. Mahram pairs auto-suggested same room
6. Occupancy grid: colored by fill %
7. Export: hotel manifest PDF + Excel

---

### MODULE 4: Transport Scheduler

1. Movement CRUD + Hajj/Umrah templates (Gelombang I/II, Umrah 9/12/17 hari — real Kemenag-shaped itineraries, not a generic 8-step template)
2. Vehicle CRUD per movement
3. Seat assignment (same drag-or-select pattern)
4. Group-based fill: one-click assign group to vehicle
5. Status updates from mobile (DEPARTED / ARRIVED)
6. Bus manifest PDF + WhatsApp text export
7. **Kloter scoping**: `movements.kloter_id` (nullable — unscoped movements
   like shared ground shuttles apply to every kloter). The monitoring
   dashboard has a per-kloter filter so several kloter running in parallel
   don't show as one mixed feed; a pilgrim only ever sees their own
   kloter's movements plus unscoped ones (never another kloter's flight).

---

### MODULE 5: Group Leader App — BUILT, as a PWA (`/leader`), not native

Built inside `apps/web`, not `apps/mobile-leader` (see the STATUS note in
§2) — authenticated via the same shared `/sign-in` as operator staff
(every leader is also a Better Auth org member; see the RBAC section in
§5), self-scoped to `groups.leader_id = current user` end-to-end
(`GroupLeaderRepository.EnsureLeaderOwnsGroup` on every call that takes a
`group_id`). Offline is the Serwist app-shell precache plus the local cache
and queue in §11, not PowerSync or a conflict-resolving sync engine.

**Screens:** My Groups → Roster, Check-In (one-tap DEPARTURE/ARRIVAL,
offline-queued), Group Chat, SOS (operator-wide alerts scoped to this
leader's own groups).

---

### MODULE 6: Pilgrim App — BUILT, as a PWA (`/pilgrim/[code]`)

**Design constraint: max 2 taps to any feature. No menus.** (kept)

**Onboarding:** not QR/6-digit — a unique link
(`/pilgrim/{app_access_code}`, an unguessable UUID) shared once, e.g. via
WhatsApp. Opening it calls `PilgrimAppService.GetMyInfo` straight to the
home screen, no login step required for the read-only surface.

**Screens:** Home (group/kloter/hotel/room info, next movement), SOS (full
screen), Chat, Schedule (kloter-scoped — a pilgrim in one Kemenag
departure batch never sees another batch's flights/transfers), Products
(read-only catalog, no checkout yet — Module 7).

**Optional Google account link:** a pilgrim can additionally link a real
Google identity from their home screen (`PilgrimAppService.LinkGoogleAccount`,
via the session-only auth lane in §5) — `pilgrims.linked_user_id`,
one pilgrim per Google identity (DB-unique). The access-code link keeps
working for everything above either way; this exists to have a verified
identity ready ahead of Module 7 (a payment can require
`linked_user_id IS NOT NULL`, the access code alone cannot).

- Auth is still primarily the `appAccessCode` in the URL — Google linking
  is additive, not a replacement.
- Service Worker source (`app/sw.ts`) is compiled by Serwist into generated
  `public/sw.js`; it precaches all `/pilgrim` and `/leader` routes and their
  build assets — see §11 for the remaining limits.
- SOS is queued offline via `localStorage`, sent on the browser's `online`
  event.

---

### MODULE 7: Digital Products & Orders

1. Product catalog CRUD
2. Order flow → `activateProduct()` stub → status update
3. Commission calculation on every order

---

### MODULE 8: Agent & Referral System

1. Agent registration (public page) → unique `referralCode`
2. Referral tracking via `?ref=CODE` on signup
3. asynq workers: payout, tier upgrade
4. Agent dashboard

---

### MODULE 9: Export Engine

1. Hotel manifest PDF (`@react-pdf/renderer`)
2. Bus manifest PDF
3. Pilgrim list Excel (SheetJS)
4. Agent earnings report PDF

---

## 7. Business Logic Rules

```
ROOM ALLOCATION:
- One room per pilgrim per hotel per season
- Cannot exceed capacity
- Gender constraint enforced (FAMILY = mahram pairs only)
- Substitution cascades ALL allocations in one transaction

SEAT ASSIGNMENT:
- One vehicle per pilgrim per movement
- Mahram pairs must be in same vehicle

SOS ALERT:
- Unacknowledged after 10min → ESCALATED → push to ALL coordinators
- Cannot be auto-resolved — must be manually marked RESOLVED
- SOS log is permanent — never delete

CHECK-IN:
- One check-in per pilgrim per movement per type (DEPARTURE | ARRIVAL)
- Timestamp immutable once created

ORDER / COMMISSION:
- platformMargin + operatorMargin + agentMargin ≤ 1.0 (validate on Product save)
- agentCommission = 0 if no agentId

SUBSTITUTION:
- isSubstituted = true blocks further substitution
- Irreversible — write AuditLog always
```

---

## 8. Coding Standards

### Go (Backend)

**3-layer rule — never mix:**
```go
// handler/ — HTTP + Connect only. No business logic. No DB.
func (h *Handler) CreatePilgrim(ctx context.Context, req *connect.Request[hajjv1.CreatePilgrimRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
    operatorID := OperatorIDFromCtx(ctx)
    pilgrim, err := h.pilgrimService.Create(ctx, operatorID, req.Msg)
    if err != nil { return nil, connectError(err) }
    return connect.NewResponse(toProto(pilgrim)), nil
}

// service/ — business logic only. No HTTP. No raw DB.
func (s *PilgrimService) Create(ctx context.Context, operatorID string, req *hajjv1.CreatePilgrimRequest) (*Pilgrim, error) {
    // validate business rules
    return s.repo.Create(ctx, operatorID, req)
}

// repository/ — DB access only. Wraps sqlc.
func (r *PilgrimRepository) Create(ctx context.Context, operatorID string, req *hajjv1.CreatePilgrimRequest) (*Pilgrim, error) {
    return r.queries.CreatePilgrim(ctx, db.CreatePilgrimParams{...})
}
```

**net/http + Connect wiring (main.go):**
```go
func main() {
    mux := http.NewServeMux()
    interceptors := connect.WithInterceptors(
        middleware.NewAuthInterceptor(os.Getenv("BETTER_AUTH_SECRET")),
        middleware.NewTenantInterceptor(),
    )
    mux.Handle(pilgrimv1connect.NewPilgrimServiceHandler(pilgrimHandler, interceptors))
    mux.Handle(sosv1connect.NewSosServiceHandler(sosHandler, interceptors))
    // ... other services

    // Public — no auth interceptor
    mux.Handle("/pilgrim-app/", pilgrimAppHandler)

    h2c := h2c.NewHandler(mux, &http2.Server{})
    go http.ListenAndServe(":8080", h2c)

    // asynq workers
    srv := asynq.NewServer(redisOpt, asynq.Config{Concurrency: 10})
    mux := asynq.NewServeMux()
    mux.HandleFunc(worker.TaskSosEscalate, worker.HandleSosEscalate)
    srv.Run(mux) // blocks
}
```

**Error handling:**
```go
var (
    ErrNotFound     = &AppError{Code: "NOT_FOUND",         Status: connect.CodeNotFound}
    ErrForbidden    = &AppError{Code: "FORBIDDEN",          Status: connect.CodePermissionDenied}
    ErrValidation   = &AppError{Code: "VALIDATION_ERROR",   Status: connect.CodeInvalidArgument}
    ErrUnauthorized = &AppError{Code: "AUTH_REQUIRED",      Status: connect.CodeUnauthenticated}
)

func connectError(err error) error {
    var appErr *AppError
    if errors.As(err, &appErr) {
        return connect.NewError(appErr.Status, err)
    }
    sentry.CaptureException(err)
    return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}
```

**Never:**
- Mix HTTP/Connect logic into service layer
- Call DB directly from handler
- Use `panic` in request handlers
- Expose raw DB errors to client
- Query without `operator_id` scope
- Use Fiber — this project uses net/http + Connect exclusively

### TypeScript (Frontend)

- `strict: true` everywhere — no exceptions
- No `any` — use generated types from `packages/proto-gen`
- All API calls through Connect client — never raw `fetch` in components

**Pass Better Auth session token to Connect:**
```typescript
// lib/transport.ts
import { createConnectTransport } from "@connectrpc/connect-web"
import { authClient } from "./auth-client"

export const transport = createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL!,
  interceptors: [
    (next) => async (req) => {
      const session = await authClient.getSession()
      if (session?.data?.session?.token) {
        req.header.set("Authorization", `Bearer ${session.data.session.token}`)
      }
      return next(req)
    },
  ],
})
```

**connect-query pattern:**
```typescript
import { useQuery, useMutation } from "@connectrpc/connect-query"
import { listPilgrims, createPilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_connect"

const { data } = useQuery(listPilgrims, { seasonId: currentSeasonId })
const { mutate } = useMutation(createPilgrim, {
  onSuccess: () => queryClient.invalidateQueries(listPilgrims),
})
```

### Naming
```
Go files:       snake_case
Go types:       PascalCase
SQL files:      snake_case
TS files:       kebab-case
TS components:  PascalCase
API routes:     kebab-case
Env vars:       SCREAMING_SNAKE_CASE
```

---

## 9. Environment Variables

> **This section previously listed several integrations that were never
> built** (Twilio/WhatsApp, Upstash-specific vars, a "Digital Product
> Partner" API) **or got renamed along the way** (Firebase collapsed to
> one `FIREBASE_SERVICE_ACCOUNT_JSON` var instead of three). The block
> below is a direct copy of the real, current `.env.example` — treat that
> file as the single source of truth going forward and this section as a
> mirror of it, not the other way around.

```bash
# Database
DATABASE_URL="postgresql://safrat:password@localhost:5432/safrat"

# Redis (tier-recalculation worker — apps/api/cmd/worker)
REDIS_URL="redis://localhost:6380"

# Better Auth (generate: openssl rand -base64 32)
BETTER_AUTH_SECRET=""
BETTER_AUTH_URL="https://app.safrat.com"

# Google Sign-In (Better Auth social provider) — Google Cloud Console >
# APIs & Services > Credentials > OAuth client ID (Web application).
# Authorized redirect URI: {BETTER_AUTH_URL}/api/auth/callback/google
GOOGLE_CLIENT_ID=""
GOOGLE_CLIENT_SECRET=""

# App URLs
NEXT_PUBLIC_APP_URL="https://app.safrat.com"
NEXT_PUBLIC_API_URL="https://api.safrat.com"

# Storage
R2_ACCOUNT_ID=""
R2_ACCESS_KEY_ID=""
R2_SECRET_ACCESS_KEY=""
R2_BUCKET_NAME=""

# Notifications — Firebase Console: Project Settings
# Service account JSON is server-only (apps/api) — Project Settings > Service Accounts > Generate new private key
FIREBASE_SERVICE_ACCOUNT_JSON=""
# Web config + Web Push certificate are client-safe (apps/web) — Project Settings > General / Cloud Messaging
NEXT_PUBLIC_FIREBASE_API_KEY=""
NEXT_PUBLIC_FIREBASE_PROJECT_ID=""
NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=""
NEXT_PUBLIC_FIREBASE_APP_ID=""
NEXT_PUBLIC_VAPID_PUBLIC_KEY=""

# Email — password reset + email verification, both link-based
# (apps/web/lib/email.ts). No-op (logged, not thrown) when unset.
SMTP_HOST="smtp.hostinger.com"
SMTP_PORT="465"
SMTP_USER=""
SMTP_PASSWORD=""
SMTP_FROM_EMAIL=""

# Observability
SENTRY_DSN=""
NEXT_PUBLIC_SENTRY_DSN=""
AXIOM_DATASET=""
AXIOM_TOKEN=""
```

---

## 10. Third-Party Integration Contracts

### asynq Workers

```go
// worker/sos_escalate.go
func HandleSosEscalate(ctx context.Context, t *asynq.Task) error {
    var p SosEscalatePayload
    json.Unmarshal(t.Payload(), &p)
    alert, _ := queries.GetSosAlert(ctx, p.AlertID)
    if alert.Status != "TRIGGERED" { return nil }
    queries.EscalateSosAlert(ctx, p.AlertID)
    return notifyAllCoordinators(ctx, alert.OperatorID, alert)
}

// Schedule in SOS handler:
task := asynq.NewTask(TaskSosEscalate, payload, asynq.ProcessIn(10*time.Minute))
client.Enqueue(task)
```

### Firebase Push (Go)
```go
func (s *NotificationService) NotifyAllCoordinators(ctx context.Context, operatorID string, data map[string]string) error {
    tokens, _ := s.repo.GetCoordinatorFCMTokens(ctx, operatorID)
    for _, token := range tokens {
        s.SendPushToToken(ctx, token, data["title"], data["body"], data)
    }
    return nil
}
```

---

## 11. PWA Architecture (as actually built — Serwist, not `next-pwa`)

> The original plan below used the `next-pwa` package, a `next.config.ts`
> `runtimeCaching` config, and an async server-component layout. None of
> that is what got built — no "Native App" column exists to compare
> against either, since Module 5/6 shipped as PWAs (§1, §6). This is the
> real architecture, both apps (`/pilgrim/[code]` and `/leader`) share it.

| Feature | Pilgrim / Leader PWA (actual) |
|---|---|
| View schedule / group info | ✅ `cachedFetch` (`lib/offline.ts`) — reads through to `localStorage`, falls back to last-cached value on failure, shows an "offline" banner |
| SOS / check-in (write actions) | ✅ `enqueueAction` + `useOfflineQueueFlush` — queued in `localStorage`, replayed on the browser's `online` event |
| Chat | ✅ polling, no streaming |
| Push notification | ✅ Firebase Cloud Messaging (`lib/firebase.ts`), independently optional/no-op when unconfigured — not tied to PWA install state |
| Offline guarantee | ⚠️ app shell + bounded 72-hour access snapshot; last-seen data and queued writes only, not a device-verified or conflict-resolving 72-hour sync guarantee |

```typescript
// app/sw.ts — source worker, compiled by Serwist during production builds.
// public/sw.js is generated and gitignored. The manifest includes every
// /pilgrim and /leader route plus build assets; runtime caching uses
// Serwist's Next.js defaults.
```

**Pilgrim PWA auth — `appAccessCode` primary, optional Google link additive:**
```typescript
// app/pilgrim/[code]/page.tsx — "use client", fetches on mount, not an
// async server layout
export default function PilgrimHomePage() {
  const { code } = useParams<{ code: string }>();
  useEffect(() => {
    cachedFetch(`pilgrim-info:${code}`, () => pilgrimAppClient.getMyInfo({ appAccessCode: code }))
      .then((result) => { if (result.data) setInfo(result.data); })
      .catch(() => setError("Kode akses tidak ditemukan..."));
  }, [code]);
  // ... optional "Hubungkan dengan Google" card if !info.linkedGoogleEmail — see §5
}
```

`/pilgrim/[code]` and `/leader` are both in `middleware.ts`'s public-path
handling differently: `/pilgrim` is fully public (no session required at
all — see §5's `publicProcedures`), while `/leader` requires a session
(any signed-in identity gets past the Edge middleware) **and** a
`RequireAccess` guard that does the real role check (§5) — middleware
alone can't verify roles since Edge middleware has no DB access.

**Test PWA (HTTPS required — use your VPS domain, not localhost):**
1. Deploy to VPS with valid SSL
2. Open `/pilgrim/[code]` on mobile → "Add to Home Screen" must appear
3. Disable WiFi → schedule still visible (Service Worker cache)
4. Press SOS with no network → reconnect → alert delivered

---

## 12. Deployment Checklist

**Infrastructure:**
- [ ] VPS provisioned (Ubuntu 22.04 LTS, min 2 vCPU / 4GB RAM)
- [ ] Docker + Docker Compose installed
- [ ] nginx installed
- [ ] DNS: `app.safrat.com` + `api.safrat.com` → VPS IP
- [ ] SSL: `certbot --nginx -d app.safrat.com -d api.safrat.com`
- [ ] GitHub Actions secrets: `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`

**Database:**
- [ ] PostgreSQL running in Docker Compose
- [ ] `DATABASE_URL` in `.env.prod`
- [ ] Better Auth tables created: `npx better-auth migrate`
- [ ] goose migrations run: `goose -dir apps/api/db/migrations postgres $DATABASE_URL up`
- [ ] Daily backup cron configured

**Application:**
- [ ] `BETTER_AUTH_SECRET` generated and set in both web and API env
- [ ] `buf generate proto/` — clean output, no manual edits to proto-gen
- [ ] Redis instance reachable, `REDIS_URL` set (self-hosted/Docker, not Upstash — see §9)
- [ ] asynq workers start with Go binary
- [ ] Firebase FCM enabled, `FIREBASE_SERVICE_ACCOUNT_JSON` in env (optional — no-op when unset)
- [ ] Cloudflare R2 bucket created, CORS configured
- [ ] VAPID keys generated, added to env
- [ ] `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET` set, redirect URI registered in Google Cloud Console (see §9)
- [ ] Hostinger `SMTP_USER` and `SMTP_PASSWORD` set (password reset, email verification, invitations, and 2FA OTP are wired — see DEPLOY.md §13 — but silently no-op without them)

**PWA:**
- [ ] PWA manifest: "Add to Home Screen" appears on Android
- [ ] Offline test: disable WiFi → schedule visible
- [ ] SOS offline queue test

**Security (full checklist: `DEPLOY.md` §13):**
- [ ] nginx sends `Strict-Transport-Security` + `Referrer-Policy` on both server blocks
- [ ] Access logging disabled for `/pilgrim/*` and `/leader/*` (UUID-in-URL — see DEPLOY.md §13)
- [ ] `NODE_ENV=production` set (enables Better Auth's own brute-force rate limiting)
- [ ] Password reset + email verification built before any Module 7 payment work ships

**End-to-end:**
- [ ] SOS end-to-end: trigger → push → acknowledge within 10 min
- [ ] Substitution: verify room + bus + group all cascade
- [ ] CSV import: 10-row test
- [ ] PDF manifest export
- [ ] `buf breaking proto/ --against '.git#branch=main'` passes

---

## 13. Key Decisions Log

| Decision | Choice | Reason |
|---|---|---|
| Backend | **Go + net/http + Connect** | 10x faster than Node, true concurrency, Connect requires net/http |
| API protocol | **Connect (gRPC-compatible)** | Single `.proto` → type-safe Go + TS; streaming for SOS/chat; browser-native |
| Auth | **Better Auth** | Open source, multi-tenant orgs built-in, ALL data in your PostgreSQL — zero external dependency. Replaces Clerk. Session strategy is opaque DB tokens, not JWT — see §5. |
| Database | **PostgreSQL 16 self-hosted** | Full control, no vendor cost, no cold starts |
| Hosting | **VPS + Docker Compose** | Own infrastructure, predictable cost, nginx handles routing |
| ~~Mobile offline: PowerSync~~ → **Serwist + local cache** | `app/sw.ts` route/build precache; `lib/offline.ts` read-through cache + offline write queue; bounded 72-hour access snapshot | Cold-start shell and last-seen data work without the network, but this remains simpler than a conflict-resolving sync engine and needs device-level offline verification — see §11 |
| Background jobs | **asynq** | Go-native, same binary, no external service. Two periodic tasks: agent tier recalc (5min), SOS escalation (1min) |
| Pilgrim auth | **appAccessCode**, + **optional Google link** | Target: 45–65yo, first-time smartphone — no password required for read-only use. Google linking (`pilgrims.linked_user_id`) added ahead of Module 7 so a verified identity exists before money changes hands — additive, not a replacement |
| ~~PWA scope: Pilgrim + Operator only~~ → **all three are PWAs** | Group Leader also shipped as a PWA (`/leader`), not native | Same reason as the offline-cache row above — no device/simulator to verify a native build; `apps/mobile-leader`/`apps/mobile-pilgrim` remain empty scaffolds for if that work resumes |
| Payout | **Monthly batch** | Simpler, industry standard, easier reconciliation |
| Staff/Leader auth | **Google Sign-In (additive)** | Better Auth `socialProviders.google`; email/password unchanged. `account_not_linked` (Better Auth default) blocks a same-email account takeover unless the existing local row is already `emailVerified` |
| Session policy | **Single-session, enforced server-side** | `databaseHooks.session.create.after` deletes every other session for that user on each new sign-in, no grace period — money will eventually flow through this login (Module 7) |
| Login routing | **RBAC-based, one shared `/sign-in`** | `IdentityService.GetMyAccess` + `resolveLandingPath()`: routes by Better Auth org-role tier (owner/admin → dashboard, member-role leader → `/leader`, linked pilgrim → `/pilgrim/[code]`), not a flat "is an org member" boolean — see §5 |
| Kloter | **New domain concept, added mid-build** | The real Kemenag departure-batch, distinct from Rombongan/Group — see Module 2/4 |

---

*Last updated: August 2026 — v1.3 (Google Sign-In + RBAC-based single login, Kloter, security audit)*
*Paired with: UI_SPEC.md · DEPLOY.md · Hajj_Umrah_SaaS_PRD.docx*
