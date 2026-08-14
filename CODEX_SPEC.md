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
| **Group Leader App** | Mutawwif / Group Leader | Mobile (React Native / Expo) — **offline-first, native only** |
| **Pilgrim App** | Pilgrim | Mobile (React Native / Expo) — **offline-first** |
| **Pilgrim PWA** | Pilgrim (no app install) | Web PWA — light version, same `appAccessCode` auth |

> **PWA strategy:** Progressive enhancement only. Operator Dashboard and Pilgrim PWA are web-based.
> Group Leader App must remain native — PWA cannot support PowerSync offline sync or reliable iOS push for SOS.

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
PWA:              next-pwa (Workbox-based Service Worker)
                  - Operator Dashboard: installable, full offline shell cache
                  - Pilgrim PWA (/pilgrim/[code]): schedule + SOS cached at login
                  - Web Push API for coordinator notifications (Android reliable, iOS 16.4+)
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
Auth:             Better Auth JWT validation
                  - Better Auth (running in Next.js) issues signed JWT
                  - Go validates JWT using BETTER_AUTH_SECRET (shared secret)
                  - Library: github.com/golang-jwt/jwt/v5
                  - Connect interceptor extracts operatorId + userId from claims
                  - No external auth service — all data in your own PostgreSQL
Validation:       protovalidate-go  (validate proto messages in Go — no separate validator)
Cache + Queue:    Redis (Upstash — serverless Redis)
                  - Rate limiting (sync, SOS)
                  - asynq job queue broker
Background jobs:  asynq  (Redis-backed, Go-native)
                  - Goroutine pool in same binary
                  - asynqmon web UI for monitoring + replay
Push notif:       Firebase Admin Go SDK
Email:            Resend (plain HTTP POST — no SDK)
WhatsApp/SMS:     Twilio REST API
File storage:     Cloudflare R2 (AWS S3-compat — aws-sdk-go-v2)
Observability:    Sentry Go SDK + Axiom (structured logging via slog)
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
| Offline sync | **PowerSync** | Works with any PostgreSQL backend including Go, 10x less code than WatermelonDB |
| Type safety | **Protocol Buffers + Connect** | Single `.proto` → type-safe Go server + TS client. No tRPC (TS-only), no OpenAPI hand-sync |
| Background jobs | **asynq + Redis** | Go-native, same binary, Redis-backed, asynqmon UI |
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

### Authentication (Connect interceptor — Better Auth JWT)
```go
// middleware/auth.go
func NewAuthInterceptor(secret string) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            token := req.Header().Get("Authorization") // "Bearer <better_auth_jwt>"
            claims, err := validateBetterAuthJWT(token, secret)
            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, err)
            }
            ctx = context.WithValue(ctx, ctxKeyOperatorID, claims.OrgID)
            ctx = context.WithValue(ctx, ctxKeyUserID, claims.Subject)
            return next(ctx, req)
        }
    }
}

func validateBetterAuthJWT(token, secret string) (*Claims, error) {
    t, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    })
    if err != nil || !t.Valid {
        return nil, fmt.Errorf("invalid token")
    }
    return t.Claims.(*Claims), nil
}

// operatorID always comes from JWT — never trust client-provided operatorId
func OperatorIDFromCtx(ctx context.Context) string { return ctx.Value(ctxKeyOperatorID).(string) }
func UserIDFromCtx(ctx context.Context) string     { return ctx.Value(ctxKeyUserID).(string) }
```

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
- `apps/web/app/(auth)/sign-in/page.tsx` — Better Auth sign in form
- `apps/web/app/(auth)/sign-up/page.tsx` — Better Auth sign up form
- `apps/web/app/onboarding/page.tsx`

**Done when:** Operator can sign up, create org, create season, see dashboard.

---

### MODULE 2: Pilgrim Management

1. Pilgrim list with search + filter (group, gender, nationality, medical flag), pagination
2. Create pilgrim form — all fields, mahram selector
3. Bulk CSV import — column mapping wizard, row validation, batch insert
4. Edit pilgrim
5. Substitution — cascade transaction (room + seat + group)
6. Pilgrim detail page

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

1. Movement CRUD + Hajj 8-movement template
2. Vehicle CRUD per movement
3. Seat assignment (same drag-or-select pattern)
4. Group-based fill: one-click assign group to vehicle
5. Status updates from mobile (DEPARTED / ARRIVED)
6. Bus manifest PDF + WhatsApp text export

---

### MODULE 5: Group Leader Mobile App (offline-first)

**PowerSync setup:** (see full schema in previous section)

**Done when:** App works 72h with zero network. Sync resumes on reconnect.

---

### MODULE 6: Pilgrim App

**Design constraint: max 2 taps to any feature. No menus.**

**Onboarding:** QR scan or 6-digit code → `PilgrimAppService.GetMyInfo` → home screen.

**Screens:** Home (group info), SOS (full screen), Chat, Schedule, Products.

**Pilgrim PWA** (web fallback — `/pilgrim/[code]`):
- No Better Auth session — auth is purely via `appAccessCode`
- Service Worker caches schedule + group info for 72h
- SOS queued offline, sent on reconnect

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

```bash
# .env — never commit. Use .env.example as template.

# Database (self-hosted PostgreSQL in Docker)
DATABASE_URL="postgresql://safrat:password@localhost:5432/safrat"
# No DIRECT_URL needed — pgx manages its own connection pool

# Better Auth — shared secret between Next.js and Go API
BETTER_AUTH_SECRET="generate_with: openssl rand -base64 32"
BETTER_AUTH_URL="https://app.safrat.com"   # canonical URL of your web app

# Cloudflare R2
R2_ACCOUNT_ID=""
R2_ACCESS_KEY_ID=""
R2_SECRET_ACCESS_KEY=""
R2_BUCKET_NAME=""
R2_PUBLIC_URL=""

# Firebase (push notifications)
FIREBASE_PROJECT_ID=""
FIREBASE_CLIENT_EMAIL=""
FIREBASE_PRIVATE_KEY=""

# Upstash Redis (asynq + rate limiting)
UPSTASH_REDIS_URL=""
UPSTASH_REDIS_TOKEN=""

# Twilio (WhatsApp)
TWILIO_ACCOUNT_SID=""
TWILIO_AUTH_TOKEN=""
TWILIO_WHATSAPP_FROM=""

# Resend (email)
RESEND_API_KEY=""

# Digital Product Partner
ROAMING_PARTNER_API_URL=""
ROAMING_PARTNER_API_KEY=""

# Web Push / PWA
NEXT_PUBLIC_VAPID_PUBLIC_KEY=""
VAPID_PRIVATE_KEY=""
VAPID_SUBJECT="mailto:admin@safrat.com"

# App URLs
NEXT_PUBLIC_APP_URL="https://app.safrat.com"
NEXT_PUBLIC_API_URL="https://api.safrat.com"
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

## 11. PWA Architecture

| Feature | Pilgrim PWA | Pilgrim Native App |
|---|---|---|
| View schedule / group info | ✅ Service Worker cache | ✅ PowerSync |
| SOS button | ✅ queued offline | ✅ |
| Chat | ✅ polling fallback | ✅ streaming |
| Push notification | ⚠️ iOS 16.4+ only | ✅ reliable |
| Offline 72h | ❌ cache only | ✅ PowerSync |

```typescript
// next.config.ts
import withPWA from "next-pwa"
export default withPWA({
  dest: "public", register: true, skipWaiting: true,
  disable: process.env.NODE_ENV === "development",
  runtimeCaching: [
    { urlPattern: /^\/pilgrim\/.*/, handler: "NetworkFirst",
      options: { cacheName: "pilgrim-pages", expiration: { maxAgeSeconds: 72 * 60 * 60 } } },
    { urlPattern: /^\/api\/.*/, handler: "NetworkOnly" },
  ],
})
```

```json
// public/manifest.json
{
  "name": "Safrat – Pilgrim",
  "short_name": "Safrat",
  "start_url": "/pilgrim",
  "display": "standalone",
  "background_color": "#fdf9f0",
  "theme_color": "#0d3d27",
  "icons": [
    { "src": "/icons/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icons/icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable" }
  ]
}
```

**Pilgrim PWA auth — appAccessCode only (no Better Auth session):**
```typescript
// app/pilgrim/[code]/layout.tsx — no auth session needed
export default async function PilgrimLayout({ children, params }) {
  const info = await pilgrimAppClient.getMyInfo({ appAccessCode: params.code })
  if (!info) redirect("/pilgrim/not-found")
  return <PilgrimProvider info={info}>{children}</PilgrimProvider>
}
```

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
- [ ] Upstash Redis instance created, env vars set
- [ ] asynq workers start with Go binary
- [ ] Firebase FCM enabled, service account key in env
- [ ] Cloudflare R2 bucket created, CORS configured
- [ ] VAPID keys generated, added to env
- [ ] PowerSync project created, sync rules scoped by `operator_id`

**PWA:**
- [ ] PWA manifest: "Add to Home Screen" appears on Android
- [ ] Offline test: disable WiFi → schedule visible
- [ ] SOS offline queue test

**End-to-end:**
- [ ] Expo EAS build configured
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
| Auth | **Better Auth** | Open source, multi-tenant orgs built-in, ALL data in your PostgreSQL — zero external dependency. Replaces Clerk. |
| Database | **PostgreSQL 16 self-hosted** | Full control, no vendor cost, no cold starts |
| Hosting | **VPS + Docker Compose** | Own infrastructure, predictable cost, nginx handles routing |
| Mobile offline | **PowerSync** | PostgreSQL → mobile sync, works with Go backend, 10x less code than WatermelonDB |
| Background jobs | **asynq** | Go-native, same binary, no external service |
| Pilgrim auth | **appAccessCode** | Target: 45–65yo, first-time smartphone — no passwords |
| PWA scope | **Pilgrim + Operator only** | Group Leader needs PowerSync + reliable iOS push — native only |
| Payout | **Monthly batch** | Simpler, industry standard, easier reconciliation |

---

*Last updated: August 2026 — v1.2 (Better Auth replaces Clerk)*
*Paired with: UI_SPEC.md · DEPLOY.md · Hajj_Umrah_SaaS_PRD.docx*
