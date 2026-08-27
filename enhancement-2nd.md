# Safrat Enhancement Phase 2 — 9router Prompt

## Context

Stack: Turborepo monorepo, Next.js 15 (apps/web, port 3131), Go Connect RPC (apps/api, port 8131),
PostgreSQL (port 5434). Migrations: goose, apps/api/db/migrations/ — table names PLURAL.
sqlc runs from apps/api/. buf runs from proto/, generates Go → apps/api/internal/gen/, TS → packages/proto-gen/.
Go 3-layer: handler/ → service/ → repository/ only. Better Auth owns sessions/orgs.
Auth middleware: apps/api/internal/middleware/auth.go — every RPC authenticated by default.
CSS vars: --color-cream-*, --color-emerald-*, --color-gold-*, --color-warm-*, --color-danger-*.

**CRITICAL RULE: Never skip a layer. Never expose raw DB errors. Always scope by operatorID.**

Migrations so far: 001–046. Next free: 047.
All existing services: accommodation, agent, broadcast, chat, group, groupleader, identity,
kloter, notification, operator, order, pilgrim, pilgrim_app, product, registration, season, sos, transport.

Settings (OperatorProfilePanel + TeamPanel) is ALREADY DONE. Do not recreate it.

---

## Module A — Analytics Dashboard

### Goal
Add `/dashboard/analytics` page with real operational metrics per season. No new service — use
existing sqlc queries + new aggregation queries. Frontend only fetches from existing RPCs where
possible; new aggregation queries added to existing services.

### A1: New sqlc aggregation queries

File: `apps/api/db/query/analytics.sql`

```sql
-- name: GetSeasonAnalytics :one
SELECT
  COUNT(DISTINCT p.id)                                                        AS total_pilgrims,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'PAID')              AS paid_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'DP')                AS dp_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'UNPAID')            AS unpaid_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.documents_passport AND p.documents_photo AND p.documents_vaccine) AS docs_complete,
  COUNT(DISTINCT p.id) FILTER (WHERE p.hotel_checked_in)                     AS checked_in_count,
  COUNT(DISTINCT ra.id)                                                       AS rooms_allocated,
  COUNT(DISTINCT sa.id)                                                       AS seats_assigned
FROM pilgrims p
LEFT JOIN room_allocations ra ON ra.pilgrim_id = p.id
LEFT JOIN seat_assignments sa ON sa.pilgrim_id = p.id
WHERE p.operator_id = @operator_id
  AND p.season_id   = @season_id;

-- name: GetAgentSeasonStats :many
SELECT
  a.name                                                            AS agent_name,
  COUNT(DISTINCT p.id)                                             AS pilgrim_count,
  a.commission_rate
FROM agents a
LEFT JOIN pilgrims p ON p.agent_id = a.id AND p.season_id = @season_id
WHERE a.operator_id = @operator_id
GROUP BY a.id, a.name, a.commission_rate
ORDER BY pilgrim_count DESC;

-- name: GetPaymentTimelineByMonth :many
SELECT
  DATE_TRUNC('month', p.created_at)::DATE AS month,
  COUNT(*) FILTER (WHERE p.payment_status = 'PAID')   AS paid,
  COUNT(*) FILTER (WHERE p.payment_status = 'DP')     AS dp,
  COUNT(*) FILTER (WHERE p.payment_status = 'UNPAID') AS unpaid
FROM pilgrims p
WHERE p.operator_id = @operator_id
  AND p.season_id   = @season_id
GROUP BY 1
ORDER BY 1;
```

### A2: Proto update — add analytics RPC to SeasonService

File: `proto/hajj/v1/season.proto` — add inside `service SeasonService`:

```protobuf
rpc GetSeasonAnalytics(GetSeasonAnalyticsRequest) returns (SeasonAnalytics);
```

Add messages at bottom of season.proto:

```protobuf
message GetSeasonAnalyticsRequest {
  string season_id = 1 [(buf.validate.field).string.min_len = 1];
}

message SeasonAnalytics {
  int64  total_pilgrims   = 1;
  int64  paid_count       = 2;
  int64  dp_count         = 3;
  int64  unpaid_count     = 4;
  int64  docs_complete    = 5;
  int64  checked_in_count = 6;
  int64  rooms_allocated  = 7;
  int64  seats_assigned   = 8;
}
```

Run: `pnpm buf:generate`

### A3: Repository — apps/api/internal/repository/analytics.go (new file)

```go
package repository

import (
	"context"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/google/uuid"
)

type AnalyticsRepository struct{ q *db.Queries }

func NewAnalyticsRepository(q *db.Queries) *AnalyticsRepository {
	return &AnalyticsRepository{q: q}
}

func (r *AnalyticsRepository) GetSeasonAnalytics(ctx context.Context, operatorID, seasonID uuid.UUID) (db.GetSeasonAnalyticsRow, error) {
	return r.q.GetSeasonAnalytics(ctx, db.GetSeasonAnalyticsParams{
		OperatorID: operatorID,
		SeasonID:   seasonID,
	})
}
```

### A4: Service — add method to apps/api/internal/service/season.go

Add import and method to existing SeasonService:

```go
func (s *SeasonService) GetSeasonAnalytics(ctx context.Context, seasonID uuid.UUID) (db.GetSeasonAnalyticsRow, error) {
	operatorID, err := middleware.OperatorIDFromCtx(ctx)
	if err != nil {
		return db.GetSeasonAnalyticsRow{}, connectError(err)
	}
	return s.analyticsRepo.GetSeasonAnalytics(ctx, operatorID, seasonID)
}
```

Add `analyticsRepo *repository.AnalyticsRepository` field to SeasonService struct and inject in constructor.

### A5: Handler — add to apps/api/internal/handler/season.go

```go
func (h *SeasonHandler) GetSeasonAnalytics(ctx context.Context, req *connect.Request[hajjv1.GetSeasonAnalyticsRequest]) (*connect.Response[hajjv1.SeasonAnalytics], error) {
	seasonID, err := uuid.Parse(req.Msg.SeasonId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	row, err := h.svc.GetSeasonAnalytics(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&hajjv1.SeasonAnalytics{
		TotalPilgrims:  row.TotalPilgrims,
		PaidCount:      row.PaidCount,
		DpCount:        row.DpCount,
		UnpaidCount:    row.UnpaidCount,
		DocsComplete:   row.DocsComplete,
		CheckedInCount: row.CheckedInCount,
		RoomsAllocated: row.RoomsAllocated,
		SeatsAssigned:  row.SeatsAssigned,
	}), nil
}
```

### A6: Wire in main.go

In `apps/api/cmd/server/main.go`, add:
```go
analyticsRepo := repository.NewAnalyticsRepository(queries)
// pass analyticsRepo into seasonService constructor
```

### A7: Frontend — apps/web/app/dashboard/(shell)/analytics/page.tsx

```tsx
"use client";
import { useEffect, useState } from "react";
import { createSeasonClient } from "@/lib/rpc";
import { SeasonAnalytics } from "@hajj-saas/proto-gen/hajj/v1/season_pb";

export default function AnalyticsPage() {
  const [seasons, setSeasons] = useState<{ id: string; name: string }[]>([]);
  const [selectedSeason, setSelectedSeason] = useState("");
  const [analytics, setAnalytics] = useState<SeasonAnalytics | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    createSeasonClient().listSeasons({}).then(r => {
      setSeasons(r.seasons);
      if (r.seasons.length > 0) setSelectedSeason(r.seasons[0].id);
    });
  }, []);

  useEffect(() => {
    if (!selectedSeason) return;
    setLoading(true);
    createSeasonClient().getSeasonAnalytics({ seasonId: selectedSeason })
      .then(r => setAnalytics(r))
      .finally(() => setLoading(false));
  }, [selectedSeason]);

  const paidPct = analytics ? Math.round((Number(analytics.paidCount) / Math.max(Number(analytics.totalPilgrims), 1)) * 100) : 0;
  const docsPct = analytics ? Math.round((Number(analytics.docsComplete) / Math.max(Number(analytics.totalPilgrims), 1)) * 100) : 0;

  return (
    <main style={{ maxWidth: 960, margin: "0 auto", padding: "32px 24px" }}>
      <p style={{ color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" }}>ANALITIK</p>
      <h1 style={{ fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: "0 0 4px" }}>Dashboard Analitik</h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 24px" }}>Metrik operasional per musim.</p>
      <div className="gold-divider" />

      {/* Season Picker */}
      <div style={{ margin: "20px 0" }}>
        <select
          value={selectedSeason}
          onChange={e => setSelectedSeason(e.target.value)}
          style={{ padding: "10px 14px", borderRadius: 8, border: "1px solid var(--color-cream-500)", background: "var(--color-cream-200)", fontSize: 14, fontFamily: "'Plus Jakarta Sans',sans-serif" }}
        >
          {seasons.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
      </div>

      {loading && <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p>}

      {analytics && !loading && (
        <>
          {/* KPI Cards */}
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: 16, marginBottom: 32 }}>
            {[
              { label: "Total Jamaah", value: analytics.totalPilgrims.toString(), color: "var(--color-emerald-900)" },
              { label: "Lunas", value: analytics.paidCount.toString(), color: "var(--color-emerald-700)" },
              { label: "DP", value: analytics.dpCount.toString(), color: "var(--color-gold-700)" },
              { label: "Belum Bayar", value: analytics.unpaidCount.toString(), color: "var(--color-danger-600)" },
              { label: "Dokumen Lengkap", value: analytics.docsComplete.toString(), color: "var(--color-emerald-700)" },
              { label: "Check-in Hotel", value: analytics.checkedInCount.toString(), color: "var(--color-emerald-800)" },
              { label: "Kamar Dialokasikan", value: analytics.roomsAllocated.toString(), color: "var(--color-warm-700)" },
              { label: "Kursi Ditugaskan", value: analytics.seatsAssigned.toString(), color: "var(--color-warm-700)" },
            ].map(card => (
              <div key={card.label} style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "20px 18px" }}>
                <p style={{ margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 }}>{card.label}</p>
                <p style={{ margin: 0, fontSize: 28, fontWeight: 700, color: card.color }}>{card.value}</p>
              </div>
            ))}
          </div>

          {/* Progress bars */}
          <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "24px", marginBottom: 24 }}>
            <h3 style={{ margin: "0 0 20px", fontSize: 16, fontWeight: 700 }}>Progres Pembayaran</h3>
            <div style={{ marginBottom: 12 }}>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, marginBottom: 4 }}>
                <span>Lunas</span><span style={{ fontWeight: 700, color: "var(--color-emerald-700)" }}>{paidPct}%</span>
              </div>
              <div style={{ height: 8, background: "var(--color-cream-300)", borderRadius: 4, overflow: "hidden" }}>
                <div style={{ width: `${paidPct}%`, height: "100%", background: "var(--color-emerald-700)", borderRadius: 4 }} />
              </div>
            </div>
            <div>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, marginBottom: 4 }}>
                <span>Dokumen Lengkap</span><span style={{ fontWeight: 700, color: "var(--color-emerald-700)" }}>{docsPct}%</span>
              </div>
              <div style={{ height: 8, background: "var(--color-cream-300)", borderRadius: 4, overflow: "hidden" }}>
                <div style={{ width: `${docsPct}%`, height: "100%", background: "var(--color-gold-500)", borderRadius: 4 }} />
              </div>
            </div>
          </div>
        </>
      )}
    </main>
  );
}
```

### A8: Add Analytics to nav in layout.tsx

In `apps/web/app/dashboard/(shell)/layout.tsx`, add to the nav array (after Reports):

```tsx
{ href: "/dashboard/analytics", label: "Analitik", icon: IconChartBar }
```

Import `IconChartBar` from `@tabler/icons-react`.

---

## Module B — Substitusi Jamaah (Pilgrim Substitution)

### Goal
Full pilgrim substitution flow: operator replaces a jamaah with a new one, all room/seat
allocations transfer atomically, audit log written, original marked `is_substituted = true`.
This is a CODEX_SPEC §7 critical business rule.

### B1: Migration 047

File: `apps/api/db/migrations/047_pilgrim_substitutions.sql`

```sql
-- +goose Up

-- Ensure is_substituted column exists on pilgrims (may be missing if not added before)
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS is_substituted BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE pilgrim_substitutions (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id       UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id         UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  original_pilgrim_id UUID      NOT NULL REFERENCES pilgrims(id),
  new_pilgrim_id    UUID        NOT NULL REFERENCES pilgrims(id),
  reason            TEXT        NOT NULL DEFAULT '',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by        TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX pilgrim_substitutions_operator_season_idx
  ON pilgrim_substitutions(operator_id, season_id);

-- +goose Down
DROP TABLE IF EXISTS pilgrim_substitutions;
ALTER TABLE pilgrims DROP COLUMN IF EXISTS is_substituted;
```

Apply: `goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up`

### B2: sqlc queries

File: `apps/api/db/query/substitution.sql`

```sql
-- name: CreateSubstitution :one
INSERT INTO pilgrim_substitutions (
  operator_id, season_id, original_pilgrim_id, new_pilgrim_id, reason, created_by
) VALUES (
  @operator_id, @season_id, @original_pilgrim_id, @new_pilgrim_id, @reason, @created_by
) RETURNING *;

-- name: ListSubstitutions :many
SELECT
  ps.*,
  op.full_name AS original_name,
  np.full_name AS new_name
FROM pilgrim_substitutions ps
JOIN pilgrims op ON op.id = ps.original_pilgrim_id
JOIN pilgrims np ON np.id = ps.new_pilgrim_id
WHERE ps.operator_id = @operator_id
  AND ps.season_id   = @season_id
ORDER BY ps.created_at DESC;

-- name: MarkPilgrimSubstituted :exec
UPDATE pilgrims SET is_substituted = true WHERE id = @id AND operator_id = @operator_id;

-- name: TransferRoomAllocations :exec
UPDATE room_allocations
SET pilgrim_id = @new_pilgrim_id
WHERE pilgrim_id = @original_pilgrim_id;

-- name: TransferSeatAssignments :exec
UPDATE seat_assignments
SET pilgrim_id = @new_pilgrim_id
WHERE pilgrim_id = @original_pilgrim_id;
```

Run: `sqlc generate` from `apps/api/`

### B3: Proto — new file proto/hajj/v1/substitution.proto

```protobuf
syntax = "proto3";

package hajj.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service SubstitutionService {
  rpc CreateSubstitution(CreateSubstitutionRequest) returns (Substitution);
  rpc ListSubstitutions(ListSubstitutionsRequest) returns (ListSubstitutionsResponse);
}

message Substitution {
  string id                   = 1;
  string original_pilgrim_id  = 2;
  string new_pilgrim_id       = 3;
  string original_name        = 4;
  string new_name             = 5;
  string reason               = 6;
  google.protobuf.Timestamp created_at = 7;
}

message CreateSubstitutionRequest {
  string season_id            = 1 [(buf.validate.field).string.min_len = 1];
  string original_pilgrim_id  = 2 [(buf.validate.field).string.min_len = 1];
  string new_pilgrim_id       = 3 [(buf.validate.field).string.min_len = 1];
  string reason               = 4;
}

message ListSubstitutionsRequest {
  string season_id = 1 [(buf.validate.field).string.min_len = 1];
}

message ListSubstitutionsResponse {
  repeated Substitution substitutions = 1;
}
```

Add to `proto/buf.gen.yaml` if not auto-discovered (it should be via wildcard `proto/hajj/v1/*.proto`).

Run: `pnpm buf:generate`

### B4: Repository — apps/api/internal/repository/substitution.go

```go
package repository

import (
	"context"
	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubstitutionRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewSubstitutionRepository(pool *pgxpool.Pool, q *db.Queries) *SubstitutionRepository {
	return &SubstitutionRepository{pool: pool, q: q}
}

// CreateSubstitution runs the full substitution atomically in a single transaction:
// 1. Marks original pilgrim as substituted (irreversible)
// 2. Transfers all room allocations to new pilgrim
// 3. Transfers all seat assignments to new pilgrim
// 4. Inserts substitution record
// All steps inside one pgx transaction — partial failure rolls back everything.
func (r *SubstitutionRepository) CreateSubstitution(
	ctx context.Context,
	operatorID, seasonID, originalID, newID uuid.UUID,
	reason, createdBy string,
) (db.PilgrimSubstitution, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return db.PilgrimSubstitution{}, err
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	if err := qtx.MarkPilgrimSubstituted(ctx, db.MarkPilgrimSubstitutedParams{
		ID: originalID, OperatorID: operatorID,
	}); err != nil {
		return db.PilgrimSubstitution{}, err
	}

	if err := qtx.TransferRoomAllocations(ctx, db.TransferRoomAllocationsParams{
		NewPilgrimID:      newID,
		OriginalPilgrimID: originalID,
	}); err != nil {
		return db.PilgrimSubstitution{}, err
	}

	if err := qtx.TransferSeatAssignments(ctx, db.TransferSeatAssignmentsParams{
		NewPilgrimID:      newID,
		OriginalPilgrimID: originalID,
	}); err != nil {
		return db.PilgrimSubstitution{}, err
	}

	sub, err := qtx.CreateSubstitution(ctx, db.CreateSubstitutionParams{
		OperatorID:        operatorID,
		SeasonID:          seasonID,
		OriginalPilgrimID: originalID,
		NewPilgrimID:      newID,
		Reason:            reason,
		CreatedBy:         createdBy,
	})
	if err != nil {
		return db.PilgrimSubstitution{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.PilgrimSubstitution{}, err
	}
	return sub, nil
}

func (r *SubstitutionRepository) ListSubstitutions(ctx context.Context, operatorID, seasonID uuid.UUID) ([]db.ListSubstitutionsRow, error) {
	return r.q.ListSubstitutions(ctx, db.ListSubstitutionsParams{
		OperatorID: operatorID,
		SeasonID:   seasonID,
	})
}
```

### B5: Service — apps/api/internal/service/substitution.go

```go
package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SubstitutionService struct {
	repo *repository.SubstitutionRepository
}

func NewSubstitutionService(repo *repository.SubstitutionRepository) *SubstitutionService {
	return &SubstitutionService{repo: repo}
}

func (s *SubstitutionService) CreateSubstitution(ctx context.Context, seasonID, originalID, newID uuid.UUID, reason string) (*hajjv1.Substitution, error) {
	operatorID, err := middleware.OperatorIDFromCtx(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	userID := middleware.UserIDFromCtx(ctx) // implement same pattern as OperatorIDFromCtx

	sub, err := s.repo.CreateSubstitution(ctx, operatorID, seasonID, originalID, newID, reason, userID)
	if err != nil {
		return nil, connectError(err)
	}
	return &hajjv1.Substitution{
		Id:               sub.ID.String(),
		OriginalPilgrimId: sub.OriginalPilgrimID.String(),
		NewPilgrimId:     sub.NewPilgrimID.String(),
		Reason:           sub.Reason,
		CreatedAt:        timestamppb.New(sub.CreatedAt.Time),
	}, nil
}

func (s *SubstitutionService) ListSubstitutions(ctx context.Context, seasonID uuid.UUID) ([]*hajjv1.Substitution, error) {
	operatorID, err := middleware.OperatorIDFromCtx(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	rows, err := s.repo.ListSubstitutions(ctx, operatorID, seasonID)
	if err != nil {
		return nil, connectError(err)
	}
	out := make([]*hajjv1.Substitution, len(rows))
	for i, r := range rows {
		out[i] = &hajjv1.Substitution{
			Id:               r.ID.String(),
			OriginalPilgrimId: r.OriginalPilgrimID.String(),
			NewPilgrimId:     r.NewPilgrimID.String(),
			OriginalName:     r.OriginalName,
			NewName:          r.NewName,
			Reason:           r.Reason,
			CreatedAt:        timestamppb.New(r.CreatedAt.Time),
		}
	}
	return out, nil
}
```

### B6: Handler — apps/api/internal/handler/substitution.go

```go
package handler

import (
	"context"
	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/service"
)

type SubstitutionHandler struct {
	svc *service.SubstitutionService
}

func NewSubstitutionHandler(svc *service.SubstitutionService) *SubstitutionHandler {
	return &SubstitutionHandler{svc: svc}
}

func (h *SubstitutionHandler) CreateSubstitution(ctx context.Context, req *connect.Request[hajjv1.CreateSubstitutionRequest]) (*connect.Response[hajjv1.Substitution], error) {
	seasonID, err := uuid.Parse(req.Msg.SeasonId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	originalID, err := uuid.Parse(req.Msg.OriginalPilgrimId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	newID, err := uuid.Parse(req.Msg.NewPilgrimId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	sub, err := h.svc.CreateSubstitution(ctx, seasonID, originalID, newID, req.Msg.Reason)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(sub), nil
}

func (h *SubstitutionHandler) ListSubstitutions(ctx context.Context, req *connect.Request[hajjv1.ListSubstitutionsRequest]) (*connect.Response[hajjv1.ListSubstitutionsResponse], error) {
	seasonID, err := uuid.Parse(req.Msg.SeasonId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	subs, err := h.svc.ListSubstitutions(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&hajjv1.ListSubstitutionsResponse{Substitutions: subs}), nil
}
```

### B7: Wire in main.go

```go
// After existing repo/service/handler setup:
substitutionRepo    := repository.NewSubstitutionRepository(pool, queries)
substitutionSvc     := service.NewSubstitutionService(substitutionRepo)
substitutionHandler := handler.NewSubstitutionHandler(substitutionSvc)

// Generated handler path follows pattern hajjv1connect.SubstitutionServiceHandler
subPath, subHandlerFn := hajjv1connect.NewSubstitutionServiceHandler(substitutionHandler, interceptors...)
mux.Handle(subPath, subHandlerFn)
```

### B8: Frontend — apps/web/app/dashboard/(shell)/pilgrims/substitution/page.tsx

```tsx
"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { createPilgrimClient, createSubstitutionClient, createSeasonClient } from "@/lib/rpc";

type Pilgrim = { id: string; fullName: string; isSubstituted: boolean };

export default function SubstitutionPage() {
  const router = useRouter();
  const [seasons, setSeasons] = useState<{ id: string; name: string }[]>([]);
  const [selectedSeason, setSelectedSeason] = useState("");
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [originalId, setOriginalId] = useState("");
  const [newId, setNewId] = useState("");
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [history, setHistory] = useState<{ id: string; originalName: string; newName: string; reason: string; createdAt: string }[]>([]);

  useEffect(() => {
    createSeasonClient().listSeasons({}).then(r => {
      setSeasons(r.seasons);
      if (r.seasons.length > 0) setSelectedSeason(r.seasons[0].id);
    });
  }, []);

  useEffect(() => {
    if (!selectedSeason) return;
    createPilgrimClient().listPilgrims({ seasonId: selectedSeason }).then(r => {
      setPilgrims(r.pilgrims.filter((p: Pilgrim) => !p.isSubstituted));
    });
    createSubstitutionClient().listSubstitutions({ seasonId: selectedSeason }).then(r => {
      setHistory(r.substitutions.map((s: { id: string; originalName: string; newName: string; reason: string; createdAt: { toDate: () => Date } }) => ({
        id: s.id,
        originalName: s.originalName,
        newName: s.newName,
        reason: s.reason,
        createdAt: s.createdAt.toDate().toLocaleDateString("id-ID"),
      })));
    });
  }, [selectedSeason]);

  const eligible = pilgrims.filter(p => p.id !== originalId);

  const submit = async () => {
    if (!originalId || !newId || !reason.trim()) { setError("Pilih jamaah asal, jamaah pengganti, dan isi alasan."); return; }
    if (originalId === newId) { setError("Jamaah asal dan pengganti tidak boleh sama."); return; }
    setSubmitting(true); setError(""); setSuccess("");
    try {
      await createSubstitutionClient().createSubstitution({ seasonId: selectedSeason, originalPilgrimId: originalId, newPilgrimId: newId, reason });
      setSuccess("Substitusi berhasil dicatat. Alokasi kamar dan kursi telah dipindahkan.");
      setOriginalId(""); setNewId(""); setReason("");
      // Refresh pilgrim list and history
      const [pr, sr] = await Promise.all([
        createPilgrimClient().listPilgrims({ seasonId: selectedSeason }),
        createSubstitutionClient().listSubstitutions({ seasonId: selectedSeason }),
      ]);
      setPilgrims(pr.pilgrims.filter((p: Pilgrim) => !p.isSubstituted));
      setHistory(sr.substitutions.map((s: { id: string; originalName: string; newName: string; reason: string; createdAt: { toDate: () => Date } }) => ({
        id: s.id, originalName: s.originalName, newName: s.newName,
        reason: s.reason, createdAt: s.createdAt.toDate().toLocaleDateString("id-ID"),
      })));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal melakukan substitusi.");
    } finally {
      setSubmitting(false);
    }
  };

  const sel: React.CSSProperties = { padding: "10px 12px", borderRadius: 8, border: "1px solid var(--color-cream-500)", background: "var(--color-cream-200)", fontSize: 14, width: "100%", fontFamily: "'Plus Jakarta Sans',sans-serif" };
  const label: React.CSSProperties = { display: "block", fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", marginBottom: 6 };

  return (
    <main style={{ maxWidth: 760, margin: "0 auto", padding: "32px 24px" }}>
      <p style={{ color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" }}>JAMAAH</p>
      <h1 style={{ fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: "4px 0" }}>Substitusi Jamaah</h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 24px" }}>Ganti jamaah beserta seluruh alokasi kamar dan kursi. Tindakan ini tidak dapat dibatalkan.</p>
      <div className="gold-divider" />

      {/* Season picker */}
      <div style={{ margin: "20px 0" }}>
        <label style={label}>Musim</label>
        <select value={selectedSeason} onChange={e => setSelectedSeason(e.target.value)} style={sel}>
          {seasons.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
      </div>

      {/* Form */}
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24, marginBottom: 24 }}>
        <h3 style={{ margin: "0 0 20px", fontSize: 16, fontWeight: 700 }}>Buat Substitusi Baru</h3>
        <div style={{ display: "grid", gap: 16 }}>
          <div>
            <label style={label}>Jamaah yang Digantikan</label>
            <select value={originalId} onChange={e => setOriginalId(e.target.value)} style={sel}>
              <option value="">-- Pilih Jamaah Asal --</option>
              {pilgrims.map(p => <option key={p.id} value={p.id}>{p.fullName}</option>)}
            </select>
          </div>
          <div>
            <label style={label}>Jamaah Pengganti</label>
            <select value={newId} onChange={e => setNewId(e.target.value)} style={sel}>
              <option value="">-- Pilih Jamaah Pengganti --</option>
              {eligible.map(p => <option key={p.id} value={p.id}>{p.fullName}</option>)}
            </select>
          </div>
          <div>
            <label style={label}>Alasan Substitusi</label>
            <textarea
              value={reason}
              onChange={e => setReason(e.target.value)}
              rows={3}
              style={{ ...sel, resize: "vertical" }}
              placeholder="Contoh: Jamaah sakit dan tidak dapat berangkat"
            />
          </div>
          {error && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          {success && <p style={{ color: "var(--color-emerald-700)", fontSize: 13, fontWeight: 600 }}>{success}</p>}
          <div style={{ background: "var(--color-danger-50, #fff5f5)", border: "1px solid var(--color-danger-200, #fecaca)", borderRadius: 8, padding: "12px 16px", fontSize: 13, color: "var(--color-danger-700, #b91c1c)" }}>
            ⚠ Substitusi bersifat permanen. Jamaah asal akan ditandai sebagai <strong>sudah disubstitusi</strong> dan tidak dapat diubah kembali.
          </div>
          <button
            onClick={submit}
            disabled={submitting || !originalId || !newId || !reason.trim()}
            style={{ height: 44, background: "var(--color-danger-600, #dc2626)", color: "#fff", border: "none", borderRadius: 8, fontWeight: 700, fontSize: 14, cursor: "pointer", fontFamily: "'Plus Jakarta Sans',sans-serif", opacity: (submitting || !originalId || !newId || !reason.trim()) ? 0.6 : 1 }}
          >
            {submitting ? "Memproses..." : "Konfirmasi Substitusi"}
          </button>
        </div>
      </div>

      {/* History */}
      {history.length > 0 && (
        <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 }}>
          <h3 style={{ margin: "0 0 16px", fontSize: 16, fontWeight: 700 }}>Riwayat Substitusi</h3>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: "1px solid var(--color-cream-400)" }}>
                {["Jamaah Asal", "Jamaah Pengganti", "Alasan", "Tanggal"].map(h => (
                  <th key={h} style={{ textAlign: "left", padding: "8px 12px", fontWeight: 700, color: "var(--color-warm-600)" }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {history.map(r => (
                <tr key={r.id} style={{ borderBottom: "1px solid var(--color-cream-300)" }}>
                  <td style={{ padding: "10px 12px" }}>{r.originalName}</td>
                  <td style={{ padding: "10px 12px" }}>{r.newName}</td>
                  <td style={{ padding: "10px 12px", color: "var(--color-warm-500)" }}>{r.reason}</td>
                  <td style={{ padding: "10px 12px", color: "var(--color-warm-400)" }}>{r.createdAt}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  );
}
```

### B9: Add Substitution link in Pilgrim dashboard

In `apps/web/app/dashboard/(shell)/pilgrims/page.tsx`, add a link/button near the top:
```tsx
<a href="/dashboard/pilgrims/substitution" style={{ display: "inline-flex", alignItems: "center", gap: 6, padding: "8px 16px", border: "1px solid var(--color-cream-500)", borderRadius: 8, fontSize: 13, color: "var(--color-warm-700)", fontWeight: 600, textDecoration: "none" }}>
  <IconReplace size={16} /> Substitusi Jamaah
</a>
```

Import `IconReplace` from `@tabler/icons-react`.

### B10: Add createSubstitutionClient to apps/web/lib/rpc.ts

Follow the same pattern as other clients:
```ts
import { SubstitutionServiceClient } from "@hajj-saas/proto-gen/hajj/v1/substitution_connect";

export function createSubstitutionClient() {
  return createClient(SubstitutionServiceClient);
}
```

---

## Module C — RBAC UI Enforcement

### Goal
Better Auth already has roles (owner / admin / member). Enforce them in the frontend so
`member` cannot access destructive actions. No backend changes needed — backend is already
multi-tenant scoped by operatorID. This is purely frontend gating.

### C1: Hook — apps/web/lib/use-my-role.ts

```ts
"use client";
import { authClient } from "@/lib/auth-client";

export type OrgRole = "owner" | "admin" | "member";

export function useMyRole(): { role: OrgRole | null; isOwner: boolean; isAdmin: boolean } {
  const { data: session } = authClient.useSession();
  // Better Auth puts the active org member role on session.session.activeOrganizationId
  // and the member object itself via useActiveMember (if available) or from listMembers.
  // Simplest: read from the session's member role field.
  const role = (session as { session?: { role?: OrgRole } } | null)?.session?.role ?? null;
  return {
    role,
    isOwner: role === "owner",
    isAdmin: role === "owner" || role === "admin",
  };
}
```

Note: If Better Auth session does not expose role directly, fetch it once from
`authClient.organization.listMembers` filtered to `session.user.id`, cache in React context.
Implement as a context provider if the hook approach doesn't work with your Better Auth version.

### C2: RoleGate component — apps/web/components/auth/RoleGate.tsx

```tsx
"use client";
import { useMyRole, OrgRole } from "@/lib/use-my-role";

interface Props {
  require: OrgRole | OrgRole[];
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export function RoleGate({ require, children, fallback = null }: Props) {
  const { role } = useMyRole();
  const allowed = Array.isArray(require) ? require.includes(role ?? "member" as OrgRole) : role === require;
  const adminOk = (require === "admin" || (Array.isArray(require) && require.includes("admin"))) && (role === "owner" || role === "admin");
  if (!allowed && !adminOk) return <>{fallback}</>;
  return <>{children}</>;
}
```

### C3: Apply RoleGate to destructive actions

In the following files, wrap destructive buttons with `<RoleGate require={["owner","admin"]}>`:

- `apps/web/app/dashboard/(shell)/pilgrims/page.tsx` — Delete pilgrim button
- `apps/web/app/dashboard/(shell)/pilgrims/substitution/page.tsx` — Confirm substitution button
- `apps/web/app/dashboard/(shell)/accommodation/page.tsx` — Delete hotel/room buttons
- `apps/web/app/dashboard/(shell)/transport/page.tsx` — Delete movement/vehicle buttons
- `apps/web/app/dashboard/(shell)/agents/page.tsx` — Delete agent button
- `apps/web/components/settings/TeamPanel.tsx` — Remove member button (only owner)

Example usage in pilgrims/page.tsx:
```tsx
import { RoleGate } from "@/components/auth/RoleGate";

// Wrap delete button:
<RoleGate require={["owner", "admin"]}>
  <button onClick={() => handleDelete(pilgrim.id)} style={dangerBtnStyle}>Hapus</button>
</RoleGate>
```

For the substitution confirm button — wrap entire form submit in `<RoleGate require={["owner","admin"]} fallback={<p style={{color:"var(--color-warm-400)",fontSize:13}}>Hanya pemilik atau admin yang dapat melakukan substitusi.</p>}>`.

---

## Module D — File Upload Dokumen Jamaah

### Goal
Replace boolean document checkboxes with actual file upload. Pilgrims can upload their own
documents from the pilgrim app; operators can upload on behalf of a pilgrim from the dashboard.
Files stored in a local directory served statically (v1 — no S3 required, easy to swap later).

### D1: Migration 048

File: `apps/api/db/migrations/048_document_files.sql`

```sql
-- +goose Up
CREATE TABLE pilgrim_documents (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id   UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id  UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  doc_type     TEXT        NOT NULL CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','OTHER')),
  file_url     TEXT        NOT NULL,
  file_name    TEXT        NOT NULL,
  uploaded_by  TEXT        NOT NULL DEFAULT 'operator',   -- 'operator' | 'pilgrim'
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX pilgrim_documents_pilgrim_idx ON pilgrim_documents(pilgrim_id);

-- +goose Down
DROP TABLE IF EXISTS pilgrim_documents;
```

Apply: `goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up`

### D2: File upload HTTP endpoint in Go (apps/api/cmd/server/main.go)

Add a plain HTTP handler (not Connect) for multipart file upload. Register before Connect handlers:

```go
// Upload handler — accepts multipart/form-data
// POST /upload/document
// Form fields: pilgrim_id (UUID), doc_type (PASSPORT|PHOTO|VACCINE|OTHER)
// Form file:   file (max 10MB)
// Returns: {"url": "/uploads/documents/<filename>"}
// Auth: same session middleware as Connect (read Bearer token, validate session)
mux.HandleFunc("/upload/document", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Validate session (reuse existing session lookup from auth middleware)
    // Extract operatorID from context (set by auth middleware wrapper)
    operatorID := r.Context().Value(middleware.OperatorIDKey).(uuid.UUID)

    if err := r.ParseMultipartForm(10 << 20); err != nil {
        http.Error(w, "file too large (max 10MB)", http.StatusBadRequest)
        return
    }

    pilgrimIDStr := r.FormValue("pilgrim_id")
    docType      := r.FormValue("doc_type")
    pilgrimID, err := uuid.Parse(pilgrimIDStr)
    if err != nil {
        http.Error(w, "invalid pilgrim_id", http.StatusBadRequest)
        return
    }

    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "missing file", http.StatusBadRequest)
        return
    }
    defer file.Close()

    // Sanitize filename, store under /var/safrat/uploads/documents/
    ext      := filepath.Ext(header.Filename)
    safeName := fmt.Sprintf("%s-%s%s", pilgrimID, uuid.New(), ext)
    uploadDir := os.Getenv("UPLOAD_DIR") // default: ./uploads/documents
    if uploadDir == "" { uploadDir = "./uploads/documents" }
    os.MkdirAll(uploadDir, 0755)

    dst, err := os.Create(filepath.Join(uploadDir, safeName))
    if err != nil {
        http.Error(w, "failed to save file", http.StatusInternalServerError)
        return
    }
    defer dst.Close()
    io.Copy(dst, file)

    fileURL := "/uploads/documents/" + safeName

    // Insert into pilgrim_documents table
    queries.CreatePilgrimDocument(r.Context(), db.CreatePilgrimDocumentParams{
        PilgrimID:  pilgrimID,
        OperatorID: operatorID,
        DocType:    docType,
        FileURL:    fileURL,
        FileName:   header.Filename,
        UploadedBy: "operator",
    })

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"url": fileURL})
})

// Serve uploaded files statically
uploadDir := os.Getenv("UPLOAD_DIR")
if uploadDir == "" { uploadDir = "./uploads/documents" }
mux.Handle("/uploads/documents/", http.StripPrefix("/uploads/documents/", http.FileServer(http.Dir(uploadDir))))
```

### D3: sqlc query for documents

File: `apps/api/db/query/document.sql`

```sql
-- name: CreatePilgrimDocument :one
INSERT INTO pilgrim_documents (pilgrim_id, operator_id, doc_type, file_url, file_name, uploaded_by)
VALUES (@pilgrim_id, @operator_id, @doc_type, @file_url, @file_name, @uploaded_by)
RETURNING *;

-- name: ListPilgrimDocuments :many
SELECT * FROM pilgrim_documents
WHERE pilgrim_id = @pilgrim_id
ORDER BY created_at DESC;

-- name: DeletePilgrimDocument :exec
DELETE FROM pilgrim_documents
WHERE id = @id AND operator_id = @operator_id;
```

Run: `sqlc generate` from `apps/api/`

### D4: Proto — add to pilgrim.proto

In `proto/hajj/v1/pilgrim.proto`, add message and RPC to PilgrimService:

```protobuf
rpc ListPilgrimDocuments(ListPilgrimDocumentsRequest) returns (ListPilgrimDocumentsResponse);
rpc DeletePilgrimDocument(DeletePilgrimDocumentRequest) returns (DeletePilgrimDocumentResponse);

message PilgrimDocument {
  string id          = 1;
  string pilgrim_id  = 2;
  string doc_type    = 3;
  string file_url    = 4;
  string file_name   = 5;
  string uploaded_by = 6;
  google.protobuf.Timestamp created_at = 7;
}

message ListPilgrimDocumentsRequest { string pilgrim_id = 1; }
message ListPilgrimDocumentsResponse { repeated PilgrimDocument documents = 1; }
message DeletePilgrimDocumentRequest { string id = 1; }
message DeletePilgrimDocumentResponse {}
```

Run: `pnpm buf:generate`

### D5: Frontend component — apps/web/components/pilgrims/DocumentUploader.tsx

```tsx
"use client";
import { useEffect, useState } from "react";
import { IconUpload, IconTrash, IconFile } from "@tabler/icons-react";
import { createPilgrimClient } from "@/lib/rpc";

const DOC_TYPES = [
  { value: "PASSPORT", label: "Paspor" },
  { value: "PHOTO",    label: "Foto" },
  { value: "VACCINE",  label: "Sertifikat Vaksin" },
  { value: "OTHER",    label: "Lainnya" },
];

export function DocumentUploader({ pilgrimId }: { pilgrimId: string }) {
  const [docs, setDocs] = useState<{ id: string; docType: string; fileUrl: string; fileName: string }[]>([]);
  const [uploading, setUploading] = useState(false);
  const [docType, setDocType] = useState("PASSPORT");
  const [error, setError] = useState("");

  const refresh = () => {
    createPilgrimClient().listPilgrimDocuments({ pilgrimId }).then(r => setDocs(r.documents));
  };
  useEffect(refresh, [pilgrimId]);

  const upload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true); setError("");
    try {
      const fd = new FormData();
      fd.append("pilgrim_id", pilgrimId);
      fd.append("doc_type", docType);
      fd.append("file", file);
      const res = await fetch("http://localhost:8131/upload/document", {
        method: "POST",
        headers: { Authorization: `Bearer ${document.cookie.match(/better-auth\.session_token=([^;]+)/)?.[1] ?? ""}` },
        body: fd,
      });
      if (!res.ok) throw new Error(await res.text());
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload gagal.");
    } finally {
      setUploading(false);
      e.target.value = "";
    }
  };

  const remove = async (id: string) => {
    await createPilgrimClient().deletePilgrimDocument({ id });
    refresh();
  };

  return (
    <div>
      <div style={{ display: "flex", gap: 8, marginBottom: 12, flexWrap: "wrap", alignItems: "center" }}>
        <select value={docType} onChange={e => setDocType(e.target.value)}
          style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid var(--color-cream-500)", background: "var(--color-cream-200)", fontSize: 13, fontFamily: "'Plus Jakarta Sans',sans-serif" }}>
          {DOC_TYPES.map(d => <option key={d.value} value={d.value}>{d.label}</option>)}
        </select>
        <label style={{ display: "inline-flex", alignItems: "center", gap: 6, padding: "8px 14px", background: "var(--color-emerald-900)", color: "#fff", borderRadius: 8, fontSize: 13, fontWeight: 600, cursor: "pointer" }}>
          <IconUpload size={15} /> {uploading ? "Mengunggah..." : "Upload File"}
          <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={upload} style={{ display: "none" }} disabled={uploading} />
        </label>
      </div>
      {error && <p style={{ color: "var(--color-danger-600)", fontSize: 12, marginBottom: 8 }}>{error}</p>}
      <div style={{ display: "grid", gap: 8 }}>
        {docs.length === 0 && <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada dokumen diunggah.</p>}
        {docs.map(doc => (
          <div key={doc.id} style={{ display: "flex", alignItems: "center", gap: 10, padding: "10px 14px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8 }}>
            <IconFile size={18} style={{ color: "var(--color-emerald-700)", flexShrink: 0 }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <p style={{ margin: 0, fontSize: 13, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{doc.fileName}</p>
              <p style={{ margin: 0, fontSize: 11, color: "var(--color-warm-400)" }}>{DOC_TYPES.find(d => d.value === doc.docType)?.label ?? doc.docType}</p>
            </div>
            <a href={`http://localhost:8131${doc.fileUrl}`} target="_blank" rel="noreferrer" style={{ fontSize: 12, color: "var(--color-emerald-700)", fontWeight: 600, textDecoration: "none" }}>Buka</a>
            <button onClick={() => remove(doc.id)} style={{ background: "none", border: "none", cursor: "pointer", color: "var(--color-danger-500)", display: "flex", alignItems: "center" }}>
              <IconTrash size={15} />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
```

### D6: Integrate DocumentUploader into Pilgrim detail

In `apps/web/app/dashboard/(shell)/pilgrims/[id]/page.tsx` (or the pilgrim detail drawer/dialog),
add a "Dokumen" section:

```tsx
import { DocumentUploader } from "@/components/pilgrims/DocumentUploader";

// Inside pilgrim detail view, add:
<div style={{ marginTop: 24 }}>
  <h3 style={{ fontSize: 15, fontWeight: 700, margin: "0 0 12px" }}>Dokumen</h3>
  <DocumentUploader pilgrimId={pilgrim.id} />
</div>
```

---

## Execution Order

Run steps in this exact order:

1. Apply migrations: `goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up`
2. `sqlc generate` (from `apps/api/`)
3. `pnpm buf:generate` (from root)
4. Implement all Go files: analytics.go (repository), season.go (add analytics method), substitution.go (repo + service + handler), document.sql query, document repo/service/handler
5. Update `apps/api/cmd/server/main.go` to wire new handlers + upload endpoint
6. Build & run Go server: `cd apps/api && go run ./cmd/server`
7. Implement all TS/React files
8. `pnpm --filter web dev`
9. Test each module in browser

## Verification Checklist

- [ ] GET /dashboard/analytics — season picker loads, KPI cards display correct counts
- [ ] Analytics payment progress bars show correct percentage
- [ ] POST substitution — original pilgrim marked is_substituted=true in DB
- [ ] POST substitution — room_allocations.pilgrim_id updated to new pilgrim
- [ ] POST substitution — seat_assignments.pilgrim_id updated to new pilgrim
- [ ] Substitution history table shows correct names
- [ ] member role user cannot see substitution confirm button (RBAC gating)
- [ ] File upload saves file to disk, returns /uploads/documents/... URL
- [ ] pilgrim_documents row inserted correctly after upload
- [ ] Uploaded file accessible at http://localhost:8131/uploads/documents/<filename>
- [ ] Document list shows uploaded files with correct doc_type label
- [ ] `go build ./...` — zero errors
- [ ] `pnpm typecheck` — zero errors
