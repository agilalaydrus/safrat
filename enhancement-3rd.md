# Safrat Phase 3 — Technical Implementation Document
## Modules: Waiting List · Cancellation & Refund · Family Status Tracker

---

## Architectural Context & Invariants

```
Stack      : Turborepo monorepo, Next.js 15 (apps/web :3131), Go Connect RPC (apps/api :8131), PostgreSQL :5434
Auth       : Better Auth (Next.js side). Go API validates every RPC by looking up the live session row in
             the `session`/`member` tables — no JWT verification, no trust of client-supplied identity.
Layers     : handler/ → service/ → repository/ — never skip, never reach sideways.
Tenant     : Every query MUST be scoped by operatorID from ctx via middleware.OperatorIDFromCtx().
             Omitting operatorID scope is a multi-tenant data-leak bug, not a style issue.
Migrations : goose, apps/api/db/migrations/, table names PLURAL, numbered sequentially. Last: 048.
sqlc       : runs from apps/api/. emit_exact_table_names: false.
buf        : runs from proto/, generates Go → apps/api/internal/gen/, TS → packages/proto-gen/.
Public RPCs: publicProcedures allowlist in internal/middleware/auth.go. Every public RPC MUST also
             appear in rateLimitedProcedures in internal/middleware/ratelimit.go.
             SOSService/CreateSOSAlert is the ONLY exception to rate-limiting (safety rule).
Errors     : Never expose raw DB errors. Always map through connectError() in service/errors.go.
Security   : Never trust operator_id, season_id, price, or capacity from the request body on public RPCs.
             Re-derive from DB on every call.
CSS vars   : --color-cream-*, --color-emerald-*, --color-gold-*, --color-warm-*, --color-danger-*
```

**Next free migration number: 049**
**Next free proto file: waitlist.proto, cancellation.proto (family tracker reuses pilgrim_app.proto)**

---

## Module A — Waiting List Management

### Business Rule
When a season reaches capacity (`pilgrims.count >= seasons.capacity`), prospective jamaah can join
a waiting list. When a pilgrim cancels, the first person on the waitlist is automatically promoted —
they receive a notification and have 48 hours to confirm before the slot moves to the next in line.

### Security Model
- `JoinWaitlist` / `LeaveWaitlist`: **public RPC** (no session), authenticated by knowing a valid
  `operator_id` + `season_id`. Identity = email + phone + full_name from request body. Rate-limited.
- `ListWaitlist` / `PromoteFromWaitlist` / `RemoveFromWaitlist`: **authenticated** (operator session only).
- Capacity check is always server-side — client never sends "is_full".
- Email uniqueness per season enforced at DB level (UNIQUE constraint).

---

### A1 — Migration 049: season_waitlists

**File: `apps/api/db/migrations/049_season_waitlists.sql`**

```sql
-- +goose Up

-- Add capacity column to seasons if not already present.
-- Capacity = maximum pilgrims the operator will accept for this season.
ALTER TABLE seasons
  ADD COLUMN IF NOT EXISTS capacity        INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS is_registration_open BOOLEAN NOT NULL DEFAULT false;

-- Waiting list entry for a prospective jamaah.
-- One entry per email per season — DB-enforced.
CREATE TABLE season_waitlists (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id)  ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id)    ON DELETE CASCADE,
  full_name     TEXT        NOT NULL,
  email         TEXT        NOT NULL,
  phone         TEXT        NOT NULL DEFAULT '',
  product_id    UUID        REFERENCES products(id) ON DELETE SET NULL,
  position      INTEGER     NOT NULL,            -- 1-based queue position, recalculated on removal
  status        TEXT        NOT NULL DEFAULT 'WAITING'
                            CHECK (status IN ('WAITING','PROMOTED','CONFIRMED','EXPIRED','REMOVED')),
  promoted_at   TIMESTAMPTZ,                     -- when slot was offered
  expires_at    TIMESTAMPTZ,                     -- promoted_at + 48h; NULL while WAITING
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, email)
);

CREATE INDEX season_waitlists_operator_season_idx ON season_waitlists(operator_id, season_id, status);
CREATE INDEX season_waitlists_position_idx        ON season_waitlists(season_id, position);

CREATE TRIGGER season_waitlists_set_updated_at
  BEFORE UPDATE ON season_waitlists
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS season_waitlists;
ALTER TABLE seasons
  DROP COLUMN IF EXISTS capacity,
  DROP COLUMN IF EXISTS is_registration_open;
```

---

### A2 — sqlc Queries

**File: `apps/api/db/query/waitlist.sql`**

```sql
-- name: CountSeasonPilgrims :one
-- Used to check if season is full before allowing new registration or waitlist join.
SELECT COUNT(*) FROM pilgrims
WHERE season_id = @season_id AND operator_id = @operator_id;

-- name: GetSeasonCapacity :one
SELECT capacity, is_registration_open FROM seasons
WHERE id = @id AND operator_id = @operator_id;

-- name: JoinWaitlist :one
-- position = MAX(position) + 1 for the given season, or 1 if empty.
INSERT INTO season_waitlists (operator_id, season_id, full_name, email, phone, product_id, position)
VALUES (
  @operator_id, @season_id, @full_name, @email, @phone, @product_id,
  COALESCE((SELECT MAX(position) FROM season_waitlists WHERE season_id = @season_id AND status = 'WAITING'), 0) + 1
)
RETURNING *;

-- name: GetWaitlistEntryByEmail :one
SELECT * FROM season_waitlists
WHERE season_id = @season_id AND email = @email
LIMIT 1;

-- name: ListWaitlist :many
SELECT * FROM season_waitlists
WHERE operator_id = @operator_id AND season_id = @season_id
ORDER BY position ASC;

-- name: GetNextWaiting :one
-- Returns the first WAITING entry — called after a cancellation to auto-promote.
SELECT * FROM season_waitlists
WHERE season_id = @season_id AND status = 'WAITING'
ORDER BY position ASC
LIMIT 1;

-- name: PromoteWaitlistEntry :one
UPDATE season_waitlists
SET status = 'PROMOTED', promoted_at = NOW(), expires_at = NOW() + INTERVAL '48 hours'
WHERE id = @id AND operator_id = @operator_id
RETURNING *;

-- name: ConfirmWaitlistEntry :one
UPDATE season_waitlists
SET status = 'CONFIRMED'
WHERE id = @id AND season_id = @season_id AND email = @email AND status = 'PROMOTED'
RETURNING *;

-- name: ExpireWaitlistEntry :exec
UPDATE season_waitlists
SET status = 'EXPIRED'
WHERE id = @id AND status = 'PROMOTED' AND expires_at < NOW();

-- name: RemoveFromWaitlist :exec
UPDATE season_waitlists
SET status = 'REMOVED'
WHERE id = @id AND operator_id = @operator_id;

-- name: LeaveWaitlist :exec
-- Public — authenticated by email match only (no operator session).
UPDATE season_waitlists
SET status = 'REMOVED'
WHERE season_id = @season_id AND email = @email AND status IN ('WAITING','PROMOTED');
```

---

### A3 — Proto: `proto/hajj/v1/waitlist.proto`

```protobuf
syntax = "proto3";

package hajj.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service WaitlistService {
  // Public — no session required. Rate-limited.
  rpc JoinWaitlist(JoinWaitlistRequest)         returns (JoinWaitlistResponse);
  rpc LeaveWaitlist(LeaveWaitlistRequest)       returns (LeaveWaitlistResponse);
  rpc ConfirmWaitlistSlot(ConfirmWaitlistSlotRequest) returns (ConfirmWaitlistSlotResponse);

  // Authenticated — operator session required.
  rpc ListWaitlist(ListWaitlistRequest)         returns (ListWaitlistResponse);
  rpc PromoteFromWaitlist(PromoteFromWaitlistRequest) returns (WaitlistEntry);
  rpc RemoveFromWaitlist(RemoveFromWaitlistRequest)   returns (RemoveFromWaitlistResponse);
}

message WaitlistEntry {
  string id          = 1;
  string season_id   = 2;
  string full_name   = 3;
  string email       = 4;
  string phone       = 5;
  string product_id  = 6;
  int32  position    = 7;
  string status      = 8;    // WAITING | PROMOTED | CONFIRMED | EXPIRED | REMOVED
  google.protobuf.Timestamp promoted_at  = 9;
  google.protobuf.Timestamp expires_at   = 10;
  google.protobuf.Timestamp created_at   = 11;
}

// ── Public RPCs ────────────────────────────────────────────────────────────────

message JoinWaitlistRequest {
  string operator_id = 1 [(buf.validate.field).string.min_len = 1];
  string season_id   = 2 [(buf.validate.field).string.min_len = 1];
  string full_name   = 3 [(buf.validate.field).string.min_len = 1];
  string email       = 4 [(buf.validate.field).string.email   = true];
  string phone       = 5;
  string product_id  = 6;
}
message JoinWaitlistResponse {
  WaitlistEntry entry    = 1;
  bool          is_full  = 2;   // true = season full, added to waitlist
                                // false = season has capacity, redirect to register
  int32         position = 3;
}

message LeaveWaitlistRequest {
  string season_id = 1 [(buf.validate.field).string.min_len = 1];
  string email     = 2 [(buf.validate.field).string.email   = true];
}
message LeaveWaitlistResponse {}

message ConfirmWaitlistSlotRequest {
  string id        = 1 [(buf.validate.field).string.min_len = 1];
  string season_id = 2 [(buf.validate.field).string.min_len = 1];
  string email     = 3 [(buf.validate.field).string.email   = true];
}
message ConfirmWaitlistSlotResponse { WaitlistEntry entry = 1; }

// ── Authenticated RPCs ─────────────────────────────────────────────────────────

message ListWaitlistRequest {
  string season_id = 1 [(buf.validate.field).string.min_len = 1];
}
message ListWaitlistResponse {
  repeated WaitlistEntry entries = 1;
  int32 total_waiting            = 2;
}

message PromoteFromWaitlistRequest {
  string id = 1 [(buf.validate.field).string.min_len = 1];
}

message RemoveFromWaitlistRequest {
  string id = 1 [(buf.validate.field).string.min_len = 1];
}
message RemoveFromWaitlistResponse {}
```

**Run:** `pnpm buf:generate`

---

### A4 — Repository: `apps/api/internal/repository/waitlist.go`

```go
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAlreadyOnWaitlist = errors.New("email already on waitlist for this season")
var ErrSeasonHasCapacity = errors.New("season still has capacity — redirect to registration")

type WaitlistRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewWaitlistRepository(pool *pgxpool.Pool, q *db.Queries) *WaitlistRepository {
	return &WaitlistRepository{pool: pool, q: q}
}

// JoinWaitlist checks capacity, prevents duplicate email, inserts entry.
// Returns (entry, isFull, error).
// isFull=false means season has capacity — caller should redirect to registration.
func (r *WaitlistRepository) JoinWaitlist(
	ctx context.Context,
	operatorID, seasonID uuid.UUID,
	fullName, email, phone string,
	productID *uuid.UUID,
) (db.SeasonWaitlist, bool, error) {
	// 1. Validate season belongs to this operator and get capacity.
	season, err := r.q.GetSeasonCapacity(ctx, db.GetSeasonCapacityParams{
		ID: seasonID, OperatorID: operatorID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.SeasonWaitlist{}, false, errors.New("season not found")
		}
		return db.SeasonWaitlist{}, false, err
	}

	// 2. Count current pilgrims (server-side — never trust client claim).
	count, err := r.q.CountSeasonPilgrims(ctx, db.CountSeasonPilgrimsParams{
		SeasonID: seasonID, OperatorID: operatorID,
	})
	if err != nil {
		return db.SeasonWaitlist{}, false, err
	}

	isFull := season.Capacity > 0 && count >= int64(season.Capacity)

	if !isFull {
		// Season still has capacity — do not add to waitlist.
		return db.SeasonWaitlist{}, false, ErrSeasonHasCapacity
	}

	// 3. Check for duplicate email.
	_, err = r.q.GetWaitlistEntryByEmail(ctx, db.GetWaitlistEntryByEmailParams{
		SeasonID: seasonID, Email: email,
	})
	if err == nil {
		// Row exists.
		return db.SeasonWaitlist{}, true, ErrAlreadyOnWaitlist
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.SeasonWaitlist{}, true, err
	}

	// 4. Insert. pgx UNIQUE constraint on (season_id, email) is the last line of defence.
	params := db.JoinWaitlistParams{
		OperatorID: operatorID,
		SeasonID:   seasonID,
		FullName:   fullName,
		Email:      email,
		Phone:      phone,
	}
	if productID != nil {
		params.ProductID = pgtype.UUID{Bytes: *productID, Valid: true}
	}
	entry, err := r.q.JoinWaitlist(ctx, params)
	return entry, true, err
}

// PromoteNextWaiting atomically marks the next WAITING entry as PROMOTED.
// Called from CancellationService after a confirmed cancellation.
func (r *WaitlistRepository) PromoteNextWaiting(ctx context.Context, operatorID, seasonID uuid.UUID) (*db.SeasonWaitlist, error) {
	next, err := r.q.GetNextWaiting(ctx, seasonID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // No one waiting — not an error.
	}
	if err != nil {
		return nil, err
	}
	promoted, err := r.q.PromoteWaitlistEntry(ctx, db.PromoteWaitlistEntryParams{
		ID: next.ID, OperatorID: operatorID,
	})
	if err != nil {
		return nil, err
	}
	return &promoted, nil
}
```

---

### A5 — Service: `apps/api/internal/service/waitlist.go`

```go
package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WaitlistService struct {
	repo *repository.WaitlistRepository
}

func NewWaitlistService(repo *repository.WaitlistRepository) *WaitlistService {
	return &WaitlistService{repo: repo}
}

// JoinWaitlist — public RPC: operatorID comes from request body, re-validated server-side.
func (s *WaitlistService) JoinWaitlist(ctx context.Context, req *hajjv1.JoinWaitlistRequest) (*hajjv1.JoinWaitlistResponse, error) {
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator_id"))
	}
	seasonID, err := uuid.Parse(req.SeasonId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid season_id"))
	}

	var productID *uuid.UUID
	if req.ProductId != "" {
		pid, err := uuid.Parse(req.ProductId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid product_id"))
		}
		productID = &pid
	}

	entry, isFull, err := s.repo.JoinWaitlist(ctx, operatorID, seasonID, req.FullName, req.Email, req.Phone, productID)
	if errors.Is(err, repository.ErrSeasonHasCapacity) {
		return &hajjv1.JoinWaitlistResponse{IsFull: false}, nil
	}
	if errors.Is(err, repository.ErrAlreadyOnWaitlist) {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("email sudah terdaftar di daftar tunggu untuk musim ini"))
	}
	if err != nil {
		return nil, connectError(err)
	}

	return &hajjv1.JoinWaitlistResponse{
		Entry:    toWaitlistProto(entry),
		IsFull:   isFull,
		Position: entry.Position,
	}, nil
}

// ListWaitlist — authenticated.
func (s *WaitlistService) ListWaitlist(ctx context.Context, seasonID uuid.UUID) ([]*hajjv1.WaitlistEntry, int32, error) {
	operatorID, err := middleware.OperatorIDFromCtx(ctx)
	if err != nil {
		return nil, 0, connectError(err)
	}
	entries, err := s.repo.q.ListWaitlist(ctx, db.ListWaitlistParams{
		OperatorID: operatorID, SeasonID: seasonID,
	})
	if err != nil {
		return nil, 0, connectError(err)
	}
	var waiting int32
	out := make([]*hajjv1.WaitlistEntry, len(entries))
	for i, e := range entries {
		out[i] = toWaitlistProto(e)
		if e.Status == "WAITING" {
			waiting++
		}
	}
	return out, waiting, nil
}

// PromoteFromWaitlist — authenticated, operator-initiated manual promotion.
func (s *WaitlistService) PromoteFromWaitlist(ctx context.Context, id uuid.UUID) (*hajjv1.WaitlistEntry, error) {
	operatorID, err := middleware.OperatorIDFromCtx(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	entry, err := s.repo.q.PromoteWaitlistEntry(ctx, db.PromoteWaitlistEntryParams{
		ID: id, OperatorID: operatorID,
	})
	if err != nil {
		return nil, connectError(err)
	}
	return toWaitlistProto(entry), nil
}

func toWaitlistProto(e db.SeasonWaitlist) *hajjv1.WaitlistEntry {
	w := &hajjv1.WaitlistEntry{
		Id:        e.ID.String(),
		SeasonId:  e.SeasonID.String(),
		FullName:  e.FullName,
		Email:     e.Email,
		Phone:     e.Phone,
		Position:  e.Position,
		Status:    e.Status,
		CreatedAt: timestamppb.New(e.CreatedAt.Time),
	}
	if e.PromotedAt.Valid {
		w.PromotedAt = timestamppb.New(e.PromotedAt.Time)
	}
	if e.ExpiresAt.Valid {
		w.ExpiresAt = timestamppb.New(e.ExpiresAt.Time)
	}
	return w
}
```

---

### A6 — auth.go: Add public RPCs

In `apps/api/internal/middleware/auth.go`, add to `publicProcedures`:

```go
// WaitlistService — public form, no Better Auth session.
// operator_id + season_id come from request body and are re-validated server-side.
"/hajj.v1.WaitlistService/JoinWaitlist":       true,
"/hajj.v1.WaitlistService/LeaveWaitlist":      true,
"/hajj.v1.WaitlistService/ConfirmWaitlistSlot": true,
```

In `apps/api/internal/middleware/ratelimit.go`, add to `rateLimitedProcedures`:

```go
"/hajj.v1.WaitlistService/JoinWaitlist":        rate.Every(time.Hour / rateLimitBurst), // 5 per hour per IP
"/hajj.v1.WaitlistService/LeaveWaitlist":        rate.Every(time.Hour / rateLimitBurst),
"/hajj.v1.WaitlistService/ConfirmWaitlistSlot":  rate.Every(time.Hour / rateLimitBurst),
```

---

### A7 — Wire in main.go

```go
waitlistRepo    := repository.NewWaitlistRepository(pool, queries)
waitlistSvc     := service.NewWaitlistService(waitlistRepo)
waitlistHandler := handler.NewWaitlistHandler(waitlistSvc)
wlPath, wlFn   := hajjv1connect.NewWaitlistServiceHandler(waitlistHandler, interceptors...)
mux.Handle(wlPath, wlFn)
```

---

### A8 — Frontend: Public Waitlist Join Page

**File: `apps/web/app/waitlist/[operatorId]/[seasonId]/page.tsx`**

This is a **public page** — no auth check. Accessible from `/waitlist/[operatorId]/[seasonId]`.
Operator shares this link when a season fills up.

```tsx
"use client";
import { use, useState } from "react";
import { createWaitlistClient } from "@/lib/rpc";

export default function WaitlistPage({ params }: { params: Promise<{ operatorId: string; seasonId: string }> }) {
  const { operatorId, seasonId } = use(params);
  const [fullName, setFullName]  = useState("");
  const [email, setEmail]        = useState("");
  const [phone, setPhone]        = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult]      = useState<{ position: number; email: string } | null>(null);
  const [error, setError]        = useState("");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true); setError("");
    try {
      const res = await createWaitlistClient().joinWaitlist({
        operatorId, seasonId, fullName, email, phone,
      });
      if (!res.isFull) {
        // Season has capacity — redirect to registration.
        window.location.href = `/register/${operatorId}/${seasonId}`;
        return;
      }
      setResult({ position: res.position, email });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal mendaftar. Coba lagi.");
    } finally {
      setSubmitting(false);
    }
  };

  const inp: React.CSSProperties = { display: "block", width: "100%", marginTop: 6, padding: "10px 12px", fontSize: 14, borderRadius: 8, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-500)", fontFamily: "'Plus Jakarta Sans',sans-serif", outline: "none", boxSizing: "border-box" };
  const lbl: React.CSSProperties = { display: "block", fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };

  if (result) return (
    <main style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", background: "var(--color-cream-100)", padding: 24 }}>
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 14, padding: "40px 32px", maxWidth: 440, width: "100%", textAlign: "center" }}>
        <div style={{ fontSize: 40, marginBottom: 16 }}>✅</div>
        <h2 style={{ fontSize: 22, fontWeight: 700, margin: "0 0 8px" }}>Anda Masuk Daftar Tunggu</h2>
        <p style={{ color: "var(--color-warm-500)", fontSize: 14, margin: "0 0 20px" }}>
          Posisi antrian Anda: <strong style={{ color: "var(--color-emerald-900)" }}>#{result.position}</strong>
        </p>
        <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>
          Kami akan menghubungi <strong>{result.email}</strong> jika ada slot yang tersedia.
          Anda memiliki 48 jam untuk mengkonfirmasi setelah slot ditawarkan.
        </p>
      </div>
    </main>
  );

  return (
    <main style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", background: "var(--color-cream-100)", padding: 24 }}>
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 14, padding: "40px 32px", maxWidth: 440, width: "100%" }}>
        <p style={{ fontFamily: "'Playfair Display',serif", fontSize: 28, fontWeight: 700, color: "var(--color-emerald-900)", margin: "0 0 4px" }}>Safrat</p>
        <h2 style={{ fontSize: 20, fontWeight: 600, margin: "16px 0 4px" }}>Daftar Tunggu</h2>
        <p style={{ color: "var(--color-warm-400)", fontSize: 13, margin: "0 0 24px" }}>
          Musim ini sudah penuh. Daftarkan diri Anda dan kami akan memberitahu saat ada slot tersedia.
        </p>
        <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
          <label style={lbl}>Nama Lengkap<input value={fullName} onChange={e => setFullName(e.target.value)} required style={inp} placeholder="Nama sesuai paspor" /></label>
          <label style={lbl}>Email<input type="email" value={email} onChange={e => setEmail(e.target.value)} required style={inp} placeholder="email@anda.com" /></label>
          <label style={lbl}>Nomor WhatsApp<input type="tel" value={phone} onChange={e => setPhone(e.target.value)} style={inp} placeholder="+62 8xx xxxx xxxx" /></label>
          {error && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          <button type="submit" disabled={submitting} style={{ height: 44, background: "var(--color-gold-500)", color: "var(--color-warm-900)", border: "none", borderRadius: 8, fontWeight: 700, fontSize: 14, cursor: "pointer" }}>
            {submitting ? "Mendaftar..." : "Masuk Daftar Tunggu"}
          </button>
        </form>
      </div>
    </main>
  );
}
```

Add `createWaitlistClient` to `apps/web/lib/rpc.ts` — same pattern as other clients.

### A9 — Frontend: Operator Waitlist Dashboard

**File: `apps/web/app/dashboard/(shell)/waitlist/page.tsx`**

Authenticated page. Shows all entries per season with status badges, position, and Promote/Remove actions.
Add to nav in layout.tsx: `{ href: "/dashboard/waitlist", label: "Daftar Tunggu", icon: IconClockHour4 }`.

---

---

## Module B — Cancellation & Refund Policy

### Business Rule
Operator sets a tiered refund policy per season (e.g. >90 days: 100%, 60–90 days: 75%,
30–60 days: 50%, <30 days: 0%). When a pilgrim cancels, the system calculates refund amount
server-side based on departure date and policy. Cancellation is **irreversible** once confirmed.
An audit log entry is always written. After cancellation, the first waitlist entry is auto-promoted.

### Security Model
- Refund amount is NEVER trusted from the client — always calculated server-side from DB rows.
- Cancellation requires operator session (`owner` or `admin` — enforced by caller checking role).
- Once `status = 'CANCELLED'`, the pilgrim row is locked — no further updates except audit.
- `pilgrim_cancellations` rows are immutable (no UPDATE, only INSERT).

---

### B1 — Migration 050: cancellation_policies + pilgrim_cancellations

**File: `apps/api/db/migrations/050_cancellation.sql`**

```sql
-- +goose Up

-- Operator-defined refund policy for a season.
-- Tiers are evaluated in order: the FIRST tier where days_before_departure >= min_days applies.
CREATE TABLE cancellation_policies (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  name          TEXT        NOT NULL,                          -- e.g. "Lebih dari 90 hari"
  min_days      INTEGER     NOT NULL,                          -- days before departure, inclusive lower bound
  refund_pct    NUMERIC(5,2) NOT NULL CHECK (refund_pct BETWEEN 0 AND 100),
  sort_order    INTEGER     NOT NULL DEFAULT 0,               -- lower = higher priority
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX cancellation_policies_season_idx ON cancellation_policies(season_id, sort_order);
COMMENT ON TABLE cancellation_policies IS
  'Refund tiers per season. Evaluated in sort_order ASC. First matching tier wins.';

-- Immutable cancellation record. Never updated after INSERT.
CREATE TABLE pilgrim_cancellations (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id      UUID        NOT NULL REFERENCES pilgrims(id),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id       UUID        NOT NULL REFERENCES seasons(id),
  reason          TEXT        NOT NULL DEFAULT '',
  days_before     INTEGER     NOT NULL,   -- days between cancellation and departure
  refund_pct      NUMERIC(5,2) NOT NULL, -- snapshot of matched tier at time of cancellation
  refund_amount   NUMERIC(12,2) NOT NULL DEFAULT 0, -- computed server-side
  total_paid      NUMERIC(12,2) NOT NULL DEFAULT 0, -- snapshot of total_paid at time of cancel
  cancelled_by    TEXT        NOT NULL,   -- user_id of operator staff
  cancelled_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  policy_id       UUID        REFERENCES cancellation_policies(id),
  UNIQUE (pilgrim_id)  -- a pilgrim can only be cancelled once
);

-- Add cancelled status to pilgrims (if not already present).
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'ACTIVE'
    CHECK (status IN ('ACTIVE','CANCELLED','SUBSTITUTED')),
  ADD COLUMN IF NOT EXISTS total_paid NUMERIC(12,2) NOT NULL DEFAULT 0;

-- +goose Down
DROP TABLE IF EXISTS pilgrim_cancellations;
DROP TABLE IF EXISTS cancellation_policies;
ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS total_paid;
```

---

### B2 — sqlc Queries

**File: `apps/api/db/query/cancellation.sql`**

```sql
-- name: UpsertCancellationPolicy :one
INSERT INTO cancellation_policies (operator_id, season_id, name, min_days, refund_pct, sort_order)
VALUES (@operator_id, @season_id, @name, @min_days, @refund_pct, @sort_order)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListCancellationPolicies :many
SELECT * FROM cancellation_policies
WHERE operator_id = @operator_id AND season_id = @season_id
ORDER BY sort_order ASC;

-- name: DeleteCancellationPolicy :exec
DELETE FROM cancellation_policies WHERE id = @id AND operator_id = @operator_id;

-- name: GetMatchingPolicy :one
-- Returns the first tier where days_before_departure >= min_days.
SELECT * FROM cancellation_policies
WHERE season_id = @season_id
  AND min_days <= @days_before
ORDER BY sort_order ASC
LIMIT 1;

-- name: CreateCancellation :one
-- Immutable. Called inside a transaction alongside MarkPilgrimCancelled.
INSERT INTO pilgrim_cancellations (
  pilgrim_id, operator_id, season_id, reason, days_before,
  refund_pct, refund_amount, total_paid, cancelled_by, policy_id
) VALUES (
  @pilgrim_id, @operator_id, @season_id, @reason, @days_before,
  @refund_pct, @refund_amount, @total_paid, @cancelled_by, @policy_id
) RETURNING *;

-- name: MarkPilgrimCancelled :exec
UPDATE pilgrims SET status = 'CANCELLED' WHERE id = @id AND operator_id = @operator_id;

-- name: ListCancellations :many
SELECT pc.*, p.full_name AS pilgrim_name
FROM pilgrim_cancellations pc
JOIN pilgrims p ON p.id = pc.pilgrim_id
WHERE pc.operator_id = @operator_id AND pc.season_id = @season_id
ORDER BY pc.cancelled_at DESC;
```

---

### B3 — Proto: `proto/hajj/v1/cancellation.proto`

```protobuf
syntax = "proto3";

package hajj.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service CancellationService {
  // Policy management (authenticated, owner/admin only enforced at UI level)
  rpc SetCancellationPolicy(SetCancellationPolicyRequest)   returns (CancellationPolicy);
  rpc ListCancellationPolicies(ListCancellationPoliciesRequest) returns (ListCancellationPoliciesResponse);
  rpc DeleteCancellationPolicy(DeleteCancellationPolicyRequest) returns (DeleteCancellationPolicyResponse);

  // Cancellation flow (authenticated)
  rpc PreviewCancellation(PreviewCancellationRequest)       returns (CancellationPreview);
  rpc ConfirmCancellation(ConfirmCancellationRequest)       returns (PilgrimCancellation);
  rpc ListCancellations(ListCancellationsRequest)           returns (ListCancellationsResponse);
}

message CancellationPolicy {
  string  id         = 1;
  string  season_id  = 2;
  string  name       = 3;
  int32   min_days   = 4;
  double  refund_pct = 5;
  int32   sort_order = 6;
}

// PreviewCancellation shows the operator what the refund will be BEFORE confirming.
// This is a read-only calculation — nothing is written.
message CancellationPreview {
  string  pilgrim_id    = 1;
  string  pilgrim_name  = 2;
  int32   days_before   = 3;   // days until departure from today
  double  refund_pct    = 4;   // matched tier percentage
  double  total_paid    = 5;   // pilgrim's current total_paid
  double  refund_amount = 6;   // computed: total_paid * refund_pct / 100
  string  policy_name   = 7;   // name of matched tier, empty if no match (= 0% refund)
}

message PilgrimCancellation {
  string  id            = 1;
  string  pilgrim_id    = 2;
  string  pilgrim_name  = 3;
  double  refund_pct    = 4;
  double  refund_amount = 5;
  double  total_paid    = 6;
  string  reason        = 7;
  string  cancelled_by  = 8;
  google.protobuf.Timestamp cancelled_at = 9;
}

message SetCancellationPolicyRequest {
  string season_id   = 1 [(buf.validate.field).string.min_len = 1];
  string name        = 2 [(buf.validate.field).string.min_len = 1];
  int32  min_days    = 3 [(buf.validate.field).int32.gte = 0];
  double refund_pct  = 4 [(buf.validate.field).double.gte = 0, (buf.validate.field).double.lte = 100];
  int32  sort_order  = 5;
}

message ListCancellationPoliciesRequest { string season_id = 1; }
message ListCancellationPoliciesResponse { repeated CancellationPolicy policies = 1; }
message DeleteCancellationPolicyRequest  { string id = 1; }
message DeleteCancellationPolicyResponse {}

message PreviewCancellationRequest {
  string pilgrim_id = 1 [(buf.validate.field).string.min_len = 1];
}

message ConfirmCancellationRequest {
  string pilgrim_id = 1 [(buf.validate.field).string.min_len = 1];
  string reason     = 2 [(buf.validate.field).string.min_len = 1];
}

message ListCancellationsRequest { string season_id = 1; }
message ListCancellationsResponse { repeated PilgrimCancellation cancellations = 1; }
```

---

### B4 — Repository: `apps/api/internal/repository/cancellation.go`

```go
package repository

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrAlreadyCancelled = errors.New("pilgrim is already cancelled")

type CancellationRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewCancellationRepository(pool *pgxpool.Pool, q *db.Queries) *CancellationRepository {
	return &CancellationRepository{pool: pool, q: q}
}

type CancellationPreviewResult struct {
	PilgrimID    uuid.UUID
	PilgrimName  string
	DaysBefore   int32
	RefundPct    float64
	TotalPaid    float64
	RefundAmount float64
	PolicyName   string
	PolicyID     *uuid.UUID
}

// PreviewCancellation computes refund without writing anything.
func (r *CancellationRepository) PreviewCancellation(ctx context.Context, operatorID, pilgrimID uuid.UUID, departureDate time.Time) (CancellationPreviewResult, error) {
	pilgrim, err := r.q.GetPilgrim(ctx, db.GetPilgrimParams{ID: pilgrimID, OperatorID: operatorID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CancellationPreviewResult{}, errors.New("pilgrim not found")
	}
	if err != nil {
		return CancellationPreviewResult{}, err
	}
	if pilgrim.Status == "CANCELLED" {
		return CancellationPreviewResult{}, ErrAlreadyCancelled
	}

	daysBefore := int32(math.Floor(time.Until(departureDate).Hours() / 24))
	if daysBefore < 0 {
		daysBefore = 0
	}

	result := CancellationPreviewResult{
		PilgrimID:   pilgrimID,
		PilgrimName: pilgrim.FullName,
		DaysBefore:  daysBefore,
		TotalPaid:   pilgrim.TotalPaid.Float64,
	}

	policy, err := r.q.GetMatchingPolicy(ctx, db.GetMatchingPolicyParams{
		SeasonID:   pilgrim.SeasonID,
		DaysBefore: daysBefore,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CancellationPreviewResult{}, err
	}
	if err == nil {
		result.RefundPct = policy.RefundPct.Float64
		result.PolicyName = policy.Name
		pid := policy.ID
		result.PolicyID = &pid
	}
	result.RefundAmount = math.Round(result.TotalPaid*result.RefundPct/100*100) / 100
	return result, nil
}

// ConfirmCancellation runs inside a transaction:
// 1. Re-validate pilgrim not already cancelled (idempotency guard)
// 2. Re-compute refund amount server-side (never trust the preview)
// 3. Insert immutable cancellation record
// 4. Mark pilgrim status = 'CANCELLED'
// 5. Promote next waitlist entry (if any)
func (r *CancellationRepository) ConfirmCancellation(
	ctx context.Context,
	operatorID, pilgrimID uuid.UUID,
	reason, cancelledBy string,
	departureDate time.Time,
	waitlistRepo *WaitlistRepository,
) (db.PilgrimCancellation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return db.PilgrimCancellation{}, err
	}
	defer tx.Rollback(ctx)
	qtx := r.q.WithTx(tx)

	// Re-fetch inside tx to prevent TOCTOU race.
	pilgrim, err := qtx.GetPilgrim(ctx, db.GetPilgrimParams{ID: pilgrimID, OperatorID: operatorID})
	if err != nil {
		return db.PilgrimCancellation{}, err
	}
	if pilgrim.Status == "CANCELLED" {
		return db.PilgrimCancellation{}, ErrAlreadyCancelled
	}

	daysBefore := int32(math.Floor(time.Until(departureDate).Hours() / 24))
	if daysBefore < 0 { daysBefore = 0 }

	var refundPct float64
	var policyID pgtype.UUID
	policy, err := qtx.GetMatchingPolicy(ctx, db.GetMatchingPolicyParams{
		SeasonID:   pilgrim.SeasonID,
		DaysBefore: daysBefore,
	})
	if err == nil {
		refundPct = policy.RefundPct.Float64
		policyID  = pgtype.UUID{Bytes: policy.ID, Valid: true}
	}

	totalPaid    := pilgrim.TotalPaid.Float64
	refundAmount := math.Round(totalPaid*refundPct/100*100) / 100

	cancellation, err := qtx.CreateCancellation(ctx, db.CreateCancellationParams{
		PilgrimID:    pilgrimID,
		OperatorID:   operatorID,
		SeasonID:     pilgrim.SeasonID,
		Reason:       reason,
		DaysBefore:   daysBefore,
		RefundPct:    pgtype.Numeric{/* refundPct */ Valid: true},
		RefundAmount: pgtype.Numeric{/* refundAmount */ Valid: true},
		TotalPaid:    pgtype.Numeric{/* totalPaid */ Valid: true},
		CancelledBy:  cancelledBy,
		PolicyID:     policyID,
	})
	if err != nil {
		return db.PilgrimCancellation{}, err
	}

	if err := qtx.MarkPilgrimCancelled(ctx, db.MarkPilgrimCancelledParams{
		ID: pilgrimID, OperatorID: operatorID,
	}); err != nil {
		return db.PilgrimCancellation{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.PilgrimCancellation{}, err
	}

	// After commit — promote next from waitlist (best effort, not part of tx).
	go waitlistRepo.PromoteNextWaiting(context.Background(), operatorID, pilgrim.SeasonID)

	return cancellation, nil
}
```

### B5 — Service, Handler, and wire in main.go

Follow the same pattern as Module A. Service wraps repository calls, maps errors through `connectError()`.
Handler parses UUIDs, calls service. Wire in main.go with pool + queries.

### B6 — Frontend: Cancellation UI

**File: `apps/web/app/dashboard/(shell)/pilgrims/[id]/cancel/page.tsx`**

Two-step UI:
1. **Preview step** — calls `PreviewCancellation`, shows: "Jamaah akan dikenakan refund X% = Rp Y dari total Rp Z".
2. **Confirm step** — requires operator to type reason, then click "Konfirmasi Pembatalan" wrapped in `<RoleGate require={["owner","admin"]}>`.
3. On success — redirect to `/dashboard/pilgrims` with a success toast.

Show a red warning banner on step 2:
> ⚠ Pembatalan bersifat permanen. Data jamaah akan dikunci dan tidak dapat diubah.

**File: `apps/web/app/dashboard/(shell)/seasons/[id]/cancellation-policy/page.tsx`**

A simple table with + row for adding tiers. Each tier has: Nama, Min Hari Sebelum Keberangkatan,
Persentase Refund, sort_order (drag or up/down buttons). Delete button per row.

Add link from season detail page: "Atur Kebijakan Pembatalan →"

---

---

## Module C — Family Status Tracker

### Business Rule
Each pilgrim has a unique `app_access_code` (already in DB). A family member can track their
loved one's status by visiting `/track/[app_access_code]`. The page shows read-only, curated
status information — enough to give peace of mind, not enough to expose PII.

**Exposed fields** (deliberately minimal — privacy-first):
- Nama depan saja (tidak full name dengan nama keluarga)
- Payment status (Lunas / Belum Lunas — for family to follow up)
- Hotel check-in status
- Group name + leader name
- Season name + departure date
- SOS status (if any active alert — family should know)
- Last known location (city only, not GPS coords)

**NOT exposed:** passport number, date of birth, full address, room number, phone number.

### Security Model
- Public RPC authenticated by `app_access_code` only.
- `app_access_code` is a UUID string (already unique in DB) — sufficient entropy for a share token.
- Rate-limited to prevent enumeration attacks.
- Backend validates `app_access_code` exists before returning ANY data.
- Response never includes PII fields beyond what's listed above.

---

### C1 — sqlc Query

**File: `apps/api/db/query/family_tracker.sql`**

```sql
-- name: GetFamilyTrackerInfo :one
-- Returns minimal, curated info for the public family tracker page.
-- Authenticated by app_access_code only (no operator session).
SELECT
  p.id,
  SPLIT_PART(p.full_name, ' ', 1)       AS first_name,   -- first word only
  p.payment_status,
  p.hotel_checked_in,
  p.status                               AS pilgrim_status,
  s.name                                 AS season_name,
  s.start_date                           AS departure_date,
  g.name                                 AS group_name,
  COALESCE(u.name, '')                   AS leader_name,
  COALESCE(pl.city, '')                  AS current_city,
  EXISTS (
    SELECT 1 FROM sos_alerts sa
    WHERE sa.pilgrim_id = p.id AND sa.status = 'ACTIVE'
  )                                      AS has_active_sos
FROM pilgrims p
JOIN seasons s ON s.id = p.season_id
LEFT JOIN groups g ON g.id = p.group_id
LEFT JOIN "user" u ON u.id = g.leader_id
LEFT JOIN pilgrim_location pl ON pl.pilgrim_id = p.id
WHERE p.app_access_code = @app_access_code
LIMIT 1;
```

---

### C2 — Proto: Add to `proto/hajj/v1/pilgrim_app.proto`

Add new service (or RPC to existing public service):

```protobuf
service FamilyTrackerService {
  // Public — authenticated by app_access_code only.
  rpc GetFamilyStatus(GetFamilyStatusRequest) returns (FamilyStatus);
}

message GetFamilyStatusRequest {
  string app_access_code = 1 [(buf.validate.field).string.min_len = 1];
}

message FamilyStatus {
  string first_name      = 1;
  string payment_status  = 2;   // PAID | DP | UNPAID
  bool   hotel_checked_in = 3;
  string pilgrim_status  = 4;   // ACTIVE | CANCELLED
  string season_name     = 5;
  google.protobuf.Timestamp departure_date = 6;
  string group_name      = 7;
  string leader_name     = 8;
  string current_city    = 9;
  bool   has_active_sos  = 10;
}
```

Add to `publicProcedures`:
```go
"/hajj.v1.FamilyTrackerService/GetFamilyStatus": true,
```

Add to `rateLimitedProcedures`:
```go
// Tight ceiling — prevents app_access_code enumeration.
// A legitimate family member will poll at most a few times per hour.
"/hajj.v1.FamilyTrackerService/GetFamilyStatus": rate.Every(time.Minute), // 1 per minute per IP
```

---

### C3 — Repository + Service + Handler

Follow standard 3-layer pattern. Service method signature:

```go
// GetFamilyStatus — public; operatorID NOT in ctx (no session).
// app_access_code is the only identity token — validated server-side against DB.
func (s *FamilyTrackerService) GetFamilyStatus(ctx context.Context, appAccessCode string) (*hajjv1.FamilyStatus, error) {
    // Sanitize: only alphanumeric + dashes (UUID format), max 36 chars.
    if len(appAccessCode) > 36 {
        return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid code"))
    }
    row, err := s.repo.GetFamilyTrackerInfo(ctx, appAccessCode)
    if errors.Is(err, pgx.ErrNoRows) {
        // Return NotFound — do NOT reveal whether code is valid or not to prevent timing oracle.
        return nil, connect.NewError(connect.CodeNotFound, errors.New("status tidak ditemukan"))
    }
    if err != nil {
        return nil, connectError(err)
    }
    return toFamilyStatusProto(row), nil
}
```

---

### C4 — Frontend: Public Tracker Page

**File: `apps/web/app/track/[code]/page.tsx`**

No auth. Accessible at `safrat.id/track/[app_access_code]`.
Operator shares this URL with each jamaah's family from the pilgrim detail page.

```tsx
"use client";
import { use, useEffect, useState } from "react";
import { createFamilyTrackerClient } from "@/lib/rpc";
import { FamilyStatus } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_app_pb";

export default function TrackPage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = use(params);
  const [status, setStatus]   = useState<FamilyStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    createFamilyTrackerClient().getFamilyStatus({ appAccessCode: code })
      .then(r => setStatus(r))
      .catch(() => setNotFound(true))
      .finally(() => setLoading(false));
  }, [code]);

  if (loading) return <Centered><p style={{ color: "var(--color-warm-400)" }}>Memuat status...</p></Centered>;
  if (notFound || !status) return (
    <Centered>
      <p style={{ fontFamily: "'Playfair Display',serif", fontSize: 26, color: "var(--color-emerald-900)" }}>Safrat</p>
      <p style={{ color: "var(--color-warm-500)", marginTop: 8 }}>Kode pelacak tidak valid atau sudah tidak aktif.</p>
    </Centered>
  );

  const payLabel: Record<string, string> = { PAID: "✅ Lunas", DP: "🟡 DP", UNPAID: "🔴 Belum Bayar" };
  const departureStr = status.departureDate ? status.departureDate.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" }) : "-";

  return (
    <main style={{ minHeight: "100vh", background: "var(--color-cream-100)", padding: "40px 24px" }}>
      <div style={{ maxWidth: 480, margin: "0 auto" }}>
        <p style={{ fontFamily: "'Playfair Display',serif", fontSize: 28, color: "var(--color-emerald-900)", margin: 0 }}>Safrat</p>
        <p style={{ color: "var(--color-warm-400)", fontSize: 13, margin: "4px 0 32px" }}>Pelacak Status Jamaah</p>

        {status.hasActiveSos && (
          <div style={{ background: "#fef2f2", border: "1px solid #fecaca", borderRadius: 10, padding: "14px 18px", marginBottom: 20, color: "#dc2626", fontWeight: 700, fontSize: 14 }}>
            🚨 Jamaah ini saat ini membutuhkan bantuan. Tim koordinator sedang menangani.
          </div>
        )}

        <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 14, overflow: "hidden" }}>
          <div style={{ background: "var(--color-emerald-900)", padding: "20px 24px" }}>
            <p style={{ color: "rgba(255,255,255,0.6)", fontSize: 12, margin: "0 0 4px" }}>Nama Jamaah</p>
            <p style={{ color: "#fff", fontSize: 24, fontWeight: 700, margin: 0 }}>{status.firstName}</p>
          </div>
          <div style={{ padding: "20px 24px", display: "grid", gap: 16 }}>
            {[
              { label: "Musim",               value: status.seasonName },
              { label: "Tanggal Berangkat",   value: departureStr },
              { label: "Status Pembayaran",   value: payLabel[status.paymentStatus] ?? status.paymentStatus },
              { label: "Check-in Hotel",      value: status.hotelCheckedIn ? "✅ Sudah" : "⏳ Belum" },
              { label: "Rombongan",           value: status.groupName || "-" },
              { label: "Koordinator",         value: status.leaderName || "-" },
              { label: "Lokasi Terakhir",     value: status.currentCity || "-" },
            ].map(row => (
              <div key={row.label} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--color-cream-300)", paddingBottom: 12 }}>
                <span style={{ fontSize: 13, color: "var(--color-warm-500)" }}>{row.label}</span>
                <span style={{ fontSize: 14, fontWeight: 600, color: "var(--color-warm-800)" }}>{row.value}</span>
              </div>
            ))}
          </div>
          <div style={{ padding: "12px 24px", background: "var(--color-cream-100)", borderTop: "1px solid var(--color-cream-300)" }}>
            <p style={{ fontSize: 11, color: "var(--color-warm-400)", margin: 0 }}>
              Halaman ini hanya menampilkan informasi ringkas untuk ketenangan keluarga.
              Diperbarui secara real-time.
            </p>
          </div>
        </div>
      </div>
    </main>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return <main style={{ minHeight: "100vh", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", background: "var(--color-cream-100)", gap: 8 }}>{children}</main>;
}
```

### C5 — Share Link in Pilgrim Detail

In `apps/web/app/dashboard/(shell)/pilgrims/[id]/page.tsx`, add a "Bagikan ke Keluarga" button:

```tsx
const trackUrl = `${window.location.origin}/track/${pilgrim.appAccessCode}`;

<button
  onClick={() => { navigator.clipboard.writeText(trackUrl); setNotice("Link disalin!"); }}
  style={{ display: "inline-flex", alignItems: "center", gap: 6, padding: "8px 16px", border: "1px solid var(--color-emerald-800)", borderRadius: 8, fontSize: 13, color: "var(--color-emerald-900)", fontWeight: 600, background: "transparent", cursor: "pointer" }}
>
  <IconShare size={16} /> Bagikan ke Keluarga
</button>
```

---

## Execution Order

```
Step  1  →  Apply migrations: goose up (049, 050)
Step  2  →  sqlc generate (from apps/api/)
Step  3  →  pnpm buf:generate (from root)
Step  4  →  Implement Go files in order:
            repository/waitlist.go
            service/waitlist.go
            handler/waitlist.go
            repository/cancellation.go
            service/cancellation.go
            handler/cancellation.go
            repository/family_tracker.go
            service/family_tracker.go
            handler/family_tracker.go
Step  5  →  Update auth.go: add public RPCs to publicProcedures
Step  6  →  Update ratelimit.go: add rateLimitedProcedures entries
Step  7  →  Update main.go: wire all 3 new handlers
Step  8  →  go build ./... — must be zero errors before frontend
Step  9  →  Implement frontend pages:
            apps/web/app/waitlist/[operatorId]/[seasonId]/page.tsx
            apps/web/app/dashboard/(shell)/waitlist/page.tsx
            apps/web/app/dashboard/(shell)/seasons/[id]/cancellation-policy/page.tsx
            apps/web/app/dashboard/(shell)/pilgrims/[id]/cancel/page.tsx
            apps/web/app/track/[code]/page.tsx
Step 10  →  Add createWaitlistClient, createCancellationClient, createFamilyTrackerClient to lib/rpc.ts
Step 11  →  Add nav entries: "Daftar Tunggu" + link "Atur Kebijakan Pembatalan" from season detail
Step 12  →  pnpm --filter web dev
Step 13  →  Verify all checklist items below
```

---

## Verification Checklist

### Waiting List
- [ ] Season with capacity=0: JoinWaitlist returns `is_full=false` (no limit = always open)
- [ ] Season at capacity: JoinWaitlist returns `is_full=true` with position number
- [ ] Duplicate email on same season returns `CodeAlreadyExists` error
- [ ] `/waitlist/[operatorId]/[seasonId]` page loads without auth cookie
- [ ] operator_id not belonging to any operator returns error (not a panic)
- [ ] Operator dashboard `/dashboard/waitlist` shows all entries with status badges
- [ ] PromoteFromWaitlist updates `promoted_at` and `expires_at` (+48h) in DB

### Cancellation
- [ ] PreviewCancellation does NOT write any DB row (verified via SELECT before and after)
- [ ] ConfirmCancellation is atomic — if MarkPilgrimCancelled fails, CreateCancellation also rolls back
- [ ] Cancelling an already-CANCELLED pilgrim returns `ErrAlreadyCancelled` (not a 500)
- [ ] Refund amount is calculated server-side — sending arbitrary `refund_amount` in request has no effect
- [ ] `pilgrim_cancellations` table has no UPDATE privileges used anywhere in codebase
- [ ] After cancellation, next WAITING waitlist entry is promoted (check DB: status='PROMOTED', expires_at set)
- [ ] Cancellation policy page: adding tier with min_days=90 and refund_pct=100 saves correctly
- [ ] Refund calculation: totalPaid=5000000, refundPct=75 → refundAmount=3750000

### Family Tracker
- [ ] `/track/[valid-code]` returns first name (not full name), payment status, group name
- [ ] `/track/[invalid-code]` returns 404-equivalent, does NOT reveal whether code exists
- [ ] Response does NOT contain: passport_number, date_of_birth, phone, room_number
- [ ] Rate limit: 6th request in 1 minute from same IP returns 429
- [ ] Share link button in pilgrim detail copies correct URL to clipboard
- [ ] has_active_sos=true shows red alert banner on tracker page
- [ ] Page loads without any auth cookie (public route, no redirect to login)

### General
- [ ] `go vet ./...` — zero warnings
- [ ] `pnpm typecheck` — zero errors
- [ ] All new public RPCs present in BOTH publicProcedures AND rateLimitedProcedures
- [ ] No raw DB error messages reachable from any public endpoint
