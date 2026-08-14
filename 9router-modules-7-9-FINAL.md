# Modules 7–9: Products, Agents, Reports — Full Stack Build Prompt

Read these files FIRST before writing a single line of code. Understand every pattern:

  proto/hajj/v1/season.proto                           ← proto pattern
  proto/buf.gen.yaml                                   ← codegen config (run from proto/)
  apps/api/sqlc.yaml                                   ← sqlc config (run from apps/api/)
  apps/api/db/migrations/019_movements.sql             ← migration pattern (plural table names, trigger)
  apps/api/db/migrations/002_updated_at_trigger.sql    ← set_updated_at() function already exists
  apps/api/db/query/movement.sql                       ← sqlc query pattern (note :many with JOINs)
  apps/api/db/query/season.sql                         ← simple query pattern
  apps/api/internal/domain/season.go                   ← domain struct pattern
  apps/api/internal/repository/season.go               ← repo pattern: pgUUID, pgTimestamp helpers, toX() converters
  apps/api/internal/service/season.go                  ← service pattern: GetByBetterAuthOrgID + serviceError()
  apps/api/internal/service/errors.go                  ← serviceError() signature
  apps/api/internal/handler/season.go                  ← handler pattern: OperatorIDFromCtx + connectError()
  apps/api/internal/handler/response.go                ← connectError() signature
  apps/api/cmd/server/main.go                          ← wiring pattern
  apps/web/lib/rpc.ts                                  ← frontend client pattern
  apps/web/app/dashboard/(shell)/layout.tsx            ← nav items
  apps/web/components/accommodation/AccommodationDashboard.tsx  ← dashboard component pattern
  apps/web/components/accommodation/HotelFormDialog.tsx         ← form dialog pattern

Critical rules observed from codebase:
  - ALL table names are PLURAL: seasons, operators, pilgrims, movements, products, agents
  - FK references: REFERENCES seasons(id), REFERENCES operators(id), REFERENCES pilgrims(id)
  - set_updated_at() trigger already exists from migration 002 — just add the trigger on new tables
  - buf generate runs from: proto/
  - sqlc generate runs from: apps/api/
  - Domain structs live in: apps/api/internal/domain/
  - Repo helpers (pgUUID, pgTimestamp, pgUUID optional) already exist in the repository package
  - Service always calls operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID) first
  - Never return raw DB errors from service — always wrap with serviceError("Method.Name", err)
  - Last migration: 021_seat_assignments.sql → next: 022, 023, 024

---

## MODULE 7 — PRODUCTS (Paket Perjalanan)

### Step 7.1 — Migration

File: apps/api/db/migrations/022_products.sql

```sql
-- +goose Up
CREATE TABLE products (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  name          TEXT        NOT NULL,
  type          TEXT        NOT NULL DEFAULT 'HAJJ' CHECK (type IN ('HAJJ','UMRAH')),
  price_idr     BIGINT      NOT NULL DEFAULT 0,
  duration_days INT         NOT NULL DEFAULT 0,
  description   TEXT        NOT NULL DEFAULT '',
  inclusions    TEXT[]      NOT NULL DEFAULT '{}',
  is_active     BOOLEAN     NOT NULL DEFAULT true,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX products_operator_season_idx ON products(operator_id, season_id);
CREATE TRIGGER products_set_updated_at
  BEFORE UPDATE ON products
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS products;
```

Apply with: goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up
(Or however migrations are applied in this project — check the README or existing Makefile.)

### Step 7.2 — sqlc Query

File: apps/api/db/query/product.sql

```sql
-- name: CreateProduct :one
INSERT INTO products (operator_id, season_id, name, type, price_idr, duration_days, description, inclusions, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products
WHERE id = $1 AND operator_id = $2;

-- name: ListProducts :many
SELECT * FROM products
WHERE operator_id = $1 AND season_id = $2
ORDER BY created_at DESC;

-- name: UpdateProduct :one
UPDATE products
SET name = $3, type = $4, price_idr = $5, duration_days = $6,
    description = $7, inclusions = $8, is_active = $9, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE id = $1 AND operator_id = $2;
```

Run: cd apps/api && sqlc generate

### Step 7.3 — Proto

File: proto/hajj/v1/product.proto

```protobuf
syntax = "proto3";
package hajj.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service ProductService {
  rpc CreateProduct(CreateProductRequest) returns (Product);
  rpc GetProduct(GetProductRequest) returns (Product);
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
  rpc UpdateProduct(UpdateProductRequest) returns (Product);
  rpc DeleteProduct(DeleteProductRequest) returns (DeleteProductResponse);
}

message Product {
  string id            = 1;
  string operator_id   = 2;
  string season_id     = 3;
  string name          = 4;
  string type          = 5;
  int64  price_idr     = 6;
  int32  duration_days = 7;
  string description   = 8;
  repeated string inclusions = 9;
  bool   is_active     = 10;
  google.protobuf.Timestamp created_at = 11;
  google.protobuf.Timestamp updated_at = 12;
}

message CreateProductRequest {
  string season_id    = 1 [(buf.validate.field).string.min_len = 1];
  string name         = 2 [(buf.validate.field).string.min_len = 1];
  string type         = 3 [(buf.validate.field).string.min_len = 1];
  int64  price_idr    = 4;
  int32  duration_days = 5;
  string description  = 6;
  repeated string inclusions = 7;
}

message GetProductRequest {
  string product_id = 1 [(buf.validate.field).string.min_len = 1];
}

message ListProductsRequest {
  string season_id = 1 [(buf.validate.field).string.min_len = 1];
}

message ListProductsResponse {
  repeated Product products = 1;
}

message UpdateProductRequest {
  string product_id    = 1 [(buf.validate.field).string.min_len = 1];
  string name          = 2 [(buf.validate.field).string.min_len = 1];
  string type          = 3 [(buf.validate.field).string.min_len = 1];
  int64  price_idr     = 4;
  int32  duration_days = 5;
  string description   = 6;
  repeated string inclusions = 7;
  bool   is_active     = 8;
}

message DeleteProductRequest {
  string product_id = 1 [(buf.validate.field).string.min_len = 1];
}

message DeleteProductResponse {}
```

Run: cd proto && buf generate

### Step 7.4 — Domain

File: apps/api/internal/domain/product.go

```go
package domain

import "time"

type Product struct {
	ID           string
	OperatorID   string
	SeasonID     string
	Name         string
	Type         string
	PriceIDR     int64
	DurationDays int32
	Description  string
	Inclusions   []string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

### Step 7.5 — Repository

File: apps/api/internal/repository/product.go

```go
package repository

import (
	"context"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProductRepository struct {
	queries *db.Queries
}

func NewProductRepository(queries *db.Queries) *ProductRepository {
	return &ProductRepository{queries: queries}
}

func (r *ProductRepository) Create(ctx context.Context, operatorID, seasonID, name, productType, description string, priceIDR int64, durationDays int32, inclusions []string) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	sUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	p, err := r.queries.CreateProduct(ctx, db.CreateProductParams{
		OperatorID:   opUUID,
		SeasonID:     sUUID,
		Name:         name,
		Type:         productType,
		PriceIdr:     priceIDR,
		DurationDays: durationDays,
		Description:  description,
		Inclusions:   inclusions,
		IsActive:     true,
	})
	if err != nil {
		return nil, err
	}
	return toProduct(p), nil
}

func (r *ProductRepository) GetByID(ctx context.Context, operatorID, productID string) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pUUID, err := pgUUID(productID)
	if err != nil {
		return nil, err
	}
	p, err := r.queries.GetProduct(ctx, db.GetProductParams{ID: pUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return toProduct(p), nil
}

func (r *ProductRepository) ListBySeasonID(ctx context.Context, operatorID, seasonID string) ([]*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	sUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	ps, err := r.queries.ListProducts(ctx, db.ListProductsParams{OperatorID: opUUID, SeasonID: sUUID})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.Product, 0, len(ps))
	for _, p := range ps {
		results = append(results, toProduct(p))
	}
	return results, nil
}

func (r *ProductRepository) Update(ctx context.Context, operatorID, productID, name, productType, description string, priceIDR int64, durationDays int32, inclusions []string, isActive bool) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pUUID, err := pgUUID(productID)
	if err != nil {
		return nil, err
	}
	p, err := r.queries.UpdateProduct(ctx, db.UpdateProductParams{
		ID: pUUID, OperatorID: opUUID,
		Name: name, Type: productType, PriceIdr: priceIDR,
		DurationDays: durationDays, Description: description,
		Inclusions: inclusions, IsActive: isActive,
	})
	if err != nil {
		return nil, err
	}
	return toProduct(p), nil
}

func (r *ProductRepository) Delete(ctx context.Context, operatorID, productID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pUUID, err := pgUUID(productID)
	if err != nil {
		return err
	}
	return r.queries.DeleteProduct(ctx, db.DeleteProductParams{ID: pUUID, OperatorID: opUUID})
}

func toProduct(p db.Product) *domain.Product {
	return &domain.Product{
		ID:           uuid.UUID(p.ID.Bytes).String(),
		OperatorID:   uuid.UUID(p.OperatorID.Bytes).String(),
		SeasonID:     uuid.UUID(p.SeasonID.Bytes).String(),
		Name:         p.Name,
		Type:         p.Type,
		PriceIDR:     p.PriceIdr,
		DurationDays: p.DurationDays,
		Description:  p.Description,
		Inclusions:   p.Inclusions,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt.Time,
		UpdatedAt:    p.UpdatedAt.Time,
	}
}

// pgUUID and pgTimestamp helpers are already defined in this package (repository/season.go).
// Do NOT redefine them — they are package-level helpers shared across all repository files.
```

### Step 7.6 — Service

File: apps/api/internal/service/product.go

```go
package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProductService struct {
	operatorRepository *repository.OperatorRepository
	productRepository  *repository.ProductRepository
}

func NewProductService(operatorRepository *repository.OperatorRepository, productRepository *repository.ProductRepository) *ProductService {
	return &ProductService{operatorRepository: operatorRepository, productRepository: productRepository}
}

func (s *ProductService) Create(ctx context.Context, authenticatedOrgID string, req *hajjv1.CreateProductRequest) (*hajjv1.Product, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("ProductService.Create", apperror.ErrValidation)
	}
	if req.Type != "HAJJ" && req.Type != "UMRAH" {
		return nil, serviceError("ProductService.Create", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ProductService.Create", err)
	}
	product, err := s.productRepository.Create(ctx, operator.ID, req.SeasonId, req.Name, req.Type, req.Description, req.PriceIdr, req.DurationDays, req.Inclusions)
	if err != nil {
		return nil, serviceError("ProductService.Create", err)
	}
	return productMessage(product), nil
}

func (s *ProductService) Get(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetProductRequest) (*hajjv1.Product, error) {
	if req == nil {
		return nil, serviceError("ProductService.Get", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ProductService.Get", err)
	}
	product, err := s.productRepository.GetByID(ctx, operator.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("ProductService.Get", err)
	}
	return productMessage(product), nil
}

func (s *ProductService) List(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListProductsRequest) (*hajjv1.ListProductsResponse, error) {
	if req == nil {
		return nil, serviceError("ProductService.List", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ProductService.List", err)
	}
	products, err := s.productRepository.ListBySeasonID(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("ProductService.List", err)
	}
	resp := &hajjv1.ListProductsResponse{Products: make([]*hajjv1.Product, 0, len(products))}
	for _, p := range products {
		resp.Products = append(resp.Products, productMessage(p))
	}
	return resp, nil
}

func (s *ProductService) Update(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdateProductRequest) (*hajjv1.Product, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, serviceError("ProductService.Update", apperror.ErrValidation)
	}
	if _, err := uuid.Parse(req.ProductId); err != nil {
		return nil, serviceError("ProductService.Update", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ProductService.Update", err)
	}
	product, err := s.productRepository.Update(ctx, operator.ID, req.ProductId, req.Name, req.Type, req.Description, req.PriceIdr, req.DurationDays, req.Inclusions, req.IsActive)
	if err != nil {
		return nil, serviceError("ProductService.Update", err)
	}
	return productMessage(product), nil
}

func (s *ProductService) Delete(ctx context.Context, authenticatedOrgID string, req *hajjv1.DeleteProductRequest) (*hajjv1.DeleteProductResponse, error) {
	if req == nil {
		return nil, serviceError("ProductService.Delete", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ProductService.Delete", err)
	}
	if err := s.productRepository.Delete(ctx, operator.ID, req.ProductId); err != nil {
		return nil, serviceError("ProductService.Delete", err)
	}
	return &hajjv1.DeleteProductResponse{}, nil
}

func productMessage(p *domain.Product) *hajjv1.Product {
	return &hajjv1.Product{
		Id: p.ID, OperatorId: p.OperatorID, SeasonId: p.SeasonID,
		Name: p.Name, Type: p.Type, PriceIdr: p.PriceIDR,
		DurationDays: p.DurationDays, Description: p.Description,
		Inclusions: p.Inclusions, IsActive: p.IsActive,
		CreatedAt: timestamppb.New(p.CreatedAt),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
}
```

Note: add `"github.com/hajj-saas/api/internal/domain"` to imports.

### Step 7.7 — Handler

File: apps/api/internal/handler/product.go

```go
package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type ProductHandler struct {
	productService *service.ProductService
}

func NewProductHandler(productService *service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

func (h *ProductHandler) CreateProduct(ctx context.Context, req *connect.Request[hajjv1.CreateProductRequest]) (*connect.Response[hajjv1.Product], error) {
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.productService.Create(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ProductHandler) GetProduct(ctx context.Context, req *connect.Request[hajjv1.GetProductRequest]) (*connect.Response[hajjv1.Product], error) {
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.productService.Get(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ProductHandler) ListProducts(ctx context.Context, req *connect.Request[hajjv1.ListProductsRequest]) (*connect.Response[hajjv1.ListProductsResponse], error) {
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.productService.List(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ProductHandler) UpdateProduct(ctx context.Context, req *connect.Request[hajjv1.UpdateProductRequest]) (*connect.Response[hajjv1.Product], error) {
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.productService.Update(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ProductHandler) DeleteProduct(ctx context.Context, req *connect.Request[hajjv1.DeleteProductRequest]) (*connect.Response[hajjv1.DeleteProductResponse], error) {
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.productService.Delete(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
```

### Step 7.8 — Wire main.go

In apps/api/cmd/server/main.go, add inside the pool block following the exact same pattern as existing services:

```go
productRepository := repository.NewProductRepository(queries)
productService    := service.NewProductService(operatorRepository, productRepository)
productHandler    := handler.NewProductHandler(productService)
productPath, productServiceHandler := hajjv1connect.NewProductServiceHandler(productHandler, handlerOptions...)
mux.Handle(productPath, productServiceHandler)
```

### Step 7.9 — Frontend

#### apps/web/components/products/ProductsDashboard.tsx

Full component. Key details:
- Header: eyebrow "OPERATIONS / PRODUCTS", h1 "Products", season selector, "Add Product" emerald button
- Stats row (3 cards): Total Products, Active, Total Package Value (sum of active product prices)
- Table with columns: Name, Type badge (HAJJ/UMRAH), Price (Rp formatted), Duration, Status badge, Edit + Delete buttons
- Delete: window.confirm before calling productClient.deleteProduct
- ProductFormDialog for create + edit (initial prop for edit)
- Empty state: IconShoppingCart, "No products yet", Add Product CTA
- All CSS vars from the correct set only (--color-cream-*, --color-emerald-*, --color-gold-*, --color-warm-*)
- Season selector at top — fetch seasonClient.listSeasons and productClient.listProducts({ seasonId })

Format price with:
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(priceIdr)

#### apps/web/components/products/ProductFormDialog.tsx

Right-side sheet dialog following PilgrimFormDialog pattern exactly:
- Sticky header: "PRODUCT" eyebrow, "Add product" / "Edit product" title, X close button
- Scrollable body
- Sticky footer with gold submit button
- Fields:
  - Name (required, text input)
  - Type (required, select: Haji / Umrah) — maps to "HAJJ" / "UMRAH"
  - Price IDR (required, number input, min 0, label "Price (IDR)")
  - Duration in days (required, number input, min 1)
  - Description (textarea, optional, maxLength 1000, character count)
  - Inclusions (textarea, label "Inclusions (one per line)", optional — split on "\n" on submit, join with "\n" on load)
  - Is Active (checkbox — only show on EDIT, not on create)
- Field-level errors (Record<string, string>) — same pattern as PilgrimFormDialog
- validate() checks required fields
- Escape key closes (with unsaved changes guard)
- On save: calls productClient.createProduct or productClient.updateProduct
- onSaved(name: string) callback

#### apps/web/app/dashboard/(shell)/products/page.tsx

```tsx
import ProductsDashboard from "@/components/products/ProductsDashboard";
export default function ProductsPage() { return <ProductsDashboard />; }
```

#### apps/web/lib/rpc.ts — add:

```ts
import { ProductService } from "@hajj-saas/proto-gen/hajj/v1/product_connect";
export const productClient = createClient(ProductService, transport);
```

---

## MODULE 8 — AGENTS (Agen Referral)

### Step 8.1 — Migration

File: apps/api/db/migrations/023_agents.sql

```sql
-- +goose Up
CREATE TABLE agents (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  name            TEXT        NOT NULL,
  phone           TEXT        NOT NULL DEFAULT '',
  email           TEXT        NOT NULL DEFAULT '',
  commission_rate FLOAT8      NOT NULL DEFAULT 0,
  notes           TEXT        NOT NULL DEFAULT '',
  is_active       BOOLEAN     NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX agents_operator_idx ON agents(operator_id);
CREATE TRIGGER agents_set_updated_at
  BEFORE UPDATE ON agents
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Add agent_id to pilgrims (nullable — existing rows have no agent)
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS pilgrims_agent_idx ON pilgrims(agent_id);

-- +goose Down
ALTER TABLE pilgrims DROP COLUMN IF EXISTS agent_id;
DROP TABLE IF EXISTS agents;
```

### Step 8.2 — sqlc Query

File: apps/api/db/query/agent.sql

```sql
-- name: CreateAgent :one
INSERT INTO agents (operator_id, name, phone, email, commission_rate, notes, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents
WHERE id = $1 AND operator_id = $2;

-- name: ListAgentsWithPilgrimCount :many
SELECT
  a.*,
  COUNT(p.id)::int AS pilgrim_count
FROM agents a
LEFT JOIN pilgrims p ON p.agent_id = a.id
WHERE a.operator_id = $1
GROUP BY a.id
ORDER BY a.name ASC;

-- name: UpdateAgent :one
UPDATE agents
SET name = $3, phone = $4, email = $5, commission_rate = $6,
    notes = $7, is_active = $8, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteAgent :exec
DELETE FROM agents
WHERE id = $1 AND operator_id = $2;

-- name: AssignPilgrimToAgent :exec
UPDATE pilgrims
SET agent_id = $2, updated_at = NOW()
WHERE id = $1 AND operator_id = $3;
```

Run: cd apps/api && sqlc generate

### Step 8.3 — Proto

File: proto/hajj/v1/agent.proto

```protobuf
syntax = "proto3";
package hajj.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service AgentService {
  rpc CreateAgent(CreateAgentRequest) returns (Agent);
  rpc GetAgent(GetAgentRequest) returns (Agent);
  rpc ListAgents(ListAgentsRequest) returns (ListAgentsResponse);
  rpc UpdateAgent(UpdateAgentRequest) returns (Agent);
  rpc DeleteAgent(DeleteAgentRequest) returns (DeleteAgentResponse);
}

message Agent {
  string id              = 1;
  string operator_id     = 2;
  string name            = 3;
  string phone           = 4;
  string email           = 5;
  double commission_rate = 6;
  string notes           = 7;
  bool   is_active       = 8;
  int32  pilgrim_count   = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}

message CreateAgentRequest {
  string name            = 1 [(buf.validate.field).string.min_len = 1];
  string phone           = 2;
  string email           = 3;
  double commission_rate = 4;
  string notes           = 5;
}

message GetAgentRequest {
  string agent_id = 1 [(buf.validate.field).string.min_len = 1];
}

message ListAgentsRequest {}
message ListAgentsResponse { repeated Agent agents = 1; }

message UpdateAgentRequest {
  string agent_id        = 1 [(buf.validate.field).string.min_len = 1];
  string name            = 2 [(buf.validate.field).string.min_len = 1];
  string phone           = 3;
  string email           = 4;
  double commission_rate = 5;
  string notes           = 6;
  bool   is_active       = 7;
}

message DeleteAgentRequest {
  string agent_id = 1 [(buf.validate.field).string.min_len = 1];
}

message DeleteAgentResponse {}
```

Run: cd proto && buf generate

### Step 8.4 — Domain

File: apps/api/internal/domain/agent.go

```go
package domain

import "time"

type Agent struct {
	ID             string
	OperatorID     string
	Name           string
	Phone          string
	Email          string
	CommissionRate float64
	Notes          string
	IsActive       bool
	PilgrimCount   int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
```

### Step 8.5 — Repository

File: apps/api/internal/repository/agent.go

```go
package repository

import (
	"context"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/domain"
)

type AgentRepository struct {
	queries *db.Queries
}

func NewAgentRepository(queries *db.Queries) *AgentRepository {
	return &AgentRepository{queries: queries}
}

func (r *AgentRepository) Create(ctx context.Context, operatorID, name, phone, email, notes string, commissionRate float64) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	a, err := r.queries.CreateAgent(ctx, db.CreateAgentParams{
		OperatorID: opUUID, Name: name, Phone: phone,
		Email: email, CommissionRate: commissionRate, Notes: notes, IsActive: true,
	})
	if err != nil {
		return nil, err
	}
	return toAgent(a, 0), nil
}

func (r *AgentRepository) GetByID(ctx context.Context, operatorID, agentID string) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	aUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	a, err := r.queries.GetAgent(ctx, db.GetAgentParams{ID: aUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return toAgent(a, 0), nil
}

func (r *AgentRepository) ListByOperatorID(ctx context.Context, operatorID string) ([]*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAgentsWithPilgrimCount(ctx, opUUID)
	if err != nil {
		return nil, err
	}
	results := make([]*domain.Agent, 0, len(rows))
	for _, row := range rows {
		a := toAgent(db.Agent{
			ID: row.ID, OperatorID: row.OperatorID, Name: row.Name,
			Phone: row.Phone, Email: row.Email, CommissionRate: row.CommissionRate,
			Notes: row.Notes, IsActive: row.IsActive,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}, row.PilgrimCount)
		results = append(results, a)
	}
	return results, nil
}

func (r *AgentRepository) Update(ctx context.Context, operatorID, agentID, name, phone, email, notes string, commissionRate float64, isActive bool) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	aUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	a, err := r.queries.UpdateAgent(ctx, db.UpdateAgentParams{
		ID: aUUID, OperatorID: opUUID, Name: name, Phone: phone,
		Email: email, CommissionRate: commissionRate, Notes: notes, IsActive: isActive,
	})
	if err != nil {
		return nil, err
	}
	return toAgent(a, 0), nil
}

func (r *AgentRepository) Delete(ctx context.Context, operatorID, agentID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	aUUID, err := pgUUID(agentID)
	if err != nil {
		return err
	}
	return r.queries.DeleteAgent(ctx, db.DeleteAgentParams{ID: aUUID, OperatorID: opUUID})
}

func toAgent(a db.Agent, pilgrimCount int32) *domain.Agent {
	return &domain.Agent{
		ID:             uuid.UUID(a.ID.Bytes).String(),
		OperatorID:     uuid.UUID(a.OperatorID.Bytes).String(),
		Name:           a.Name,
		Phone:          a.Phone,
		Email:          a.Email,
		CommissionRate: a.CommissionRate,
		Notes:          a.Notes,
		IsActive:       a.IsActive,
		PilgrimCount:   pilgrimCount,
		CreatedAt:      a.CreatedAt.Time,
		UpdatedAt:      a.UpdatedAt.Time,
	}
}
```

### Step 8.6 — Service

File: apps/api/internal/service/agent.go

Follow the exact same pattern as product.go. Key differences:
- No SeasonID parameter (agents are operator-level, not per-season)
- Validate commissionRate is between 0 and 100
- List calls agentRepository.ListByOperatorID (includes pilgrim_count from JOIN)

```go
package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AgentService struct {
	operatorRepository *repository.OperatorRepository
	agentRepository    *repository.AgentRepository
}

func NewAgentService(operatorRepository *repository.OperatorRepository, agentRepository *repository.AgentRepository) *AgentService {
	return &AgentService{operatorRepository: operatorRepository, agentRepository: agentRepository}
}

func (s *AgentService) Create(ctx context.Context, authenticatedOrgID string, req *hajjv1.CreateAgentRequest) (*hajjv1.Agent, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, serviceError("AgentService.Create", apperror.ErrValidation)
	}
	if req.CommissionRate < 0 || req.CommissionRate > 100 {
		return nil, serviceError("AgentService.Create", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.Create", err)
	}
	agent, err := s.agentRepository.Create(ctx, operator.ID, req.Name, req.Phone, req.Email, req.Notes, req.CommissionRate)
	if err != nil {
		return nil, serviceError("AgentService.Create", err)
	}
	return agentMessage(agent), nil
}

func (s *AgentService) Get(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetAgentRequest) (*hajjv1.Agent, error) {
	if req == nil {
		return nil, serviceError("AgentService.Get", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.Get", err)
	}
	agent, err := s.agentRepository.GetByID(ctx, operator.ID, req.AgentId)
	if err != nil {
		return nil, serviceError("AgentService.Get", err)
	}
	return agentMessage(agent), nil
}

func (s *AgentService) List(ctx context.Context, authenticatedOrgID string) (*hajjv1.ListAgentsResponse, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.List", err)
	}
	agents, err := s.agentRepository.ListByOperatorID(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("AgentService.List", err)
	}
	resp := &hajjv1.ListAgentsResponse{Agents: make([]*hajjv1.Agent, 0, len(agents))}
	for _, a := range agents {
		resp.Agents = append(resp.Agents, agentMessage(a))
	}
	return resp, nil
}

func (s *AgentService) Update(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdateAgentRequest) (*hajjv1.Agent, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, serviceError("AgentService.Update", apperror.ErrValidation)
	}
	if _, err := uuid.Parse(req.AgentId); err != nil {
		return nil, serviceError("AgentService.Update", apperror.ErrValidation)
	}
	if req.CommissionRate < 0 || req.CommissionRate > 100 {
		return nil, serviceError("AgentService.Update", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.Update", err)
	}
	agent, err := s.agentRepository.Update(ctx, operator.ID, req.AgentId, req.Name, req.Phone, req.Email, req.Notes, req.CommissionRate, req.IsActive)
	if err != nil {
		return nil, serviceError("AgentService.Update", err)
	}
	return agentMessage(agent), nil
}

func (s *AgentService) Delete(ctx context.Context, authenticatedOrgID string, req *hajjv1.DeleteAgentRequest) (*hajjv1.DeleteAgentResponse, error) {
	if req == nil {
		return nil, serviceError("AgentService.Delete", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.Delete", err)
	}
	if err := s.agentRepository.Delete(ctx, operator.ID, req.AgentId); err != nil {
		return nil, serviceError("AgentService.Delete", err)
	}
	return &hajjv1.DeleteAgentResponse{}, nil
}

func agentMessage(a *domain.Agent) *hajjv1.Agent {
	return &hajjv1.Agent{
		Id: a.ID, OperatorId: a.OperatorID, Name: a.Name,
		Phone: a.Phone, Email: a.Email, CommissionRate: a.CommissionRate,
		Notes: a.Notes, IsActive: a.IsActive, PilgrimCount: a.PilgrimCount,
		CreatedAt: timestamppb.New(a.CreatedAt),
		UpdatedAt: timestamppb.New(a.UpdatedAt),
	}
}
```

### Step 8.7 — Handler

File: apps/api/internal/handler/agent.go

Implement AgentServiceHandler following the exact same pattern as product.go handler above.
Methods: CreateAgent, GetAgent, ListAgents, UpdateAgent, DeleteAgent.
ListAgents: `req *connect.Request[hajjv1.ListAgentsRequest]` — pass no req.Msg fields to service (operator scoped only).

### Step 8.8 — Wire main.go

Inside the pool block, add after the product wiring:

```go
agentRepository := repository.NewAgentRepository(queries)
agentService    := service.NewAgentService(operatorRepository, agentRepository)
agentHandler    := handler.NewAgentHandler(agentService)
agentPath, agentServiceHandler := hajjv1connect.NewAgentServiceHandler(agentHandler, handlerOptions...)
mux.Handle(agentPath, agentServiceHandler)
```

### Step 8.9 — Frontend

#### apps/web/components/agents/AgentsDashboard.tsx

Full component:
- Header: eyebrow "OPERATIONS / AGENTS", h1 "Agents", "Add Agent" emerald button
- Stats row (3 cards): Total Agents, Active Agents, Total Pilgrims Referred
- Agent cards grid (2 columns on desktop, 1 on mobile):
  Each card has:
    - Agent name (Playfair 18px), commission rate badge (gold: "5.00%")
    - Phone + email in small text with icons (IconPhone, IconMail)
    - Pilgrim count: "{n} jamaah dirujuk" in emerald-900
    - Active/Inactive status badge
    - Edit button (ghost) + Delete button (ghost, red text) in card footer
- Delete: window.confirm("Delete agent {name}? This cannot be undone.") then agentClient.deleteAgent
- AgentFormDialog for create and edit
- Empty state: IconUserDollar 48px, "No agents yet", explanation text, "Add Agent" gold CTA
- Fetch: agentClient.listAgents({}) on mount

#### apps/web/components/agents/AgentFormDialog.tsx

Right-side sheet dialog following PilgrimFormDialog pattern (sticky header + scrollable body + sticky footer):
- Eyebrow: "AGENT", title: "Add agent" / "Edit agent"
- X close button (circle icon)
- Fields:
  - Name (required, text input)
  - Phone (optional, placeholder "+62 812 3456 7890")
  - Email (optional, email input type)
  - Commission Rate % (optional, number input, step 0.01, min 0, max 100, placeholder "e.g. 5.00")
  - Notes (textarea, optional, maxLength 500)
  - Is Active (checkbox — only show on EDIT)
- Field-level errors map
- validate(): name required, commission 0–100, email format if provided
- Escape key + unsaved changes guard
- onSaved(name: string) callback

#### apps/web/app/dashboard/(shell)/agents/page.tsx

```tsx
import AgentsDashboard from "@/components/agents/AgentsDashboard";
export default function AgentsPage() { return <AgentsDashboard />; }
```

#### apps/web/lib/rpc.ts — add:

```ts
import { AgentService } from "@hajj-saas/proto-gen/hajj/v1/agent_connect";
export const agentClient = createClient(AgentService, transport);
```

---

## MODULE 9 — REPORTS (Laporan & Ekspor)

No new backend. All data comes from existing RPC clients. Reports are generated client-side.

### Step 9.1 — Frontend Only

#### apps/web/components/reports/ReportsDashboard.tsx

Full component. No new API clients needed — imports: pilgrimClient, accommodationClient, transportClient, agentClient.

State:
```ts
const [seasonId, setSeasonId]           = useState("");
const [seasons, setSeasons]             = useState<Season[]>([]);
const [activeReport, setActiveReport]   = useState<string | null>(null);
const [loading, setLoading]             = useState(false);
const [previewRows, setPreviewRows]     = useState<string[][]>([]);
const [previewHeaders, setPreviewHeaders] = useState<string[]>([]);
const [reportLabel, setReportLabel]     = useState("");
const [filename, setFilename]           = useState("");
const [movementId, setMovementId]       = useState("");
const [movements, setMovements]         = useState<Movement[]>([]);
```

On mount: seasonClient.listSeasons → setSeasons, setSeasonId (active or first).
When seasonId changes: transportClient.listMovements({ seasonId }) → setMovements (needed for report 3 movement picker).

Layout:
- Header: eyebrow "OPERATIONS / REPORTS", h1 "Reports", season selector
- 2×2 grid of report cards (or stacked on mobile)
- Below grid: preview section (visible when activeReport is set)

Report card spec (identical structure for all 4):
```tsx
<article style={reportCard}>
  <div style={iconWrap}><IconXxx size={20} color="var(--color-emerald-800)" /></div>
  <h2 style={cardTitle}>{title}</h2>
  <p style={cardDesc}>{description}</p>
  {extraControls}  {/* only for report 3: movement selector */}
  <button onClick={() => generate(reportKey)} style={goldBtn} disabled={loading || !seasonId}>
    {loading && activeReport === reportKey ? "Generating..." : "Generate Report"}
  </button>
</article>
```

Four reports:

REPORT 1 — "Pilgrim Manifest"
  description: "Complete pilgrim list with status for this season"
  generate():
    fetch: await pilgrimClient.listPilgrims({ seasonId, limit: 1000, offset: 0 })
    headers: ["No.", "Full Name", "Passport No.", "Nationality", "Gender", "Date of Birth", "Phone", "Group Code", "Wheelchair", "Status"]
    rows: pilgrims.pilgrims.map((p, i) => [
      String(i + 1),
      p.fullName, p.passportNumber, p.nationality,
      p.gender === Gender.FEMALE ? "Female" : "Male",
      p.dateOfBirth ? new Date(p.dateOfBirth.toDate()).toLocaleDateString("id-ID") : "",
      p.phone, p.groupId || "-",
      p.requiresWheelchair ? "Yes" : "No",
      p.isSubstituted ? "Substituted" : p.appAccessCode ? "Active" : "No Access"
    ])
    filename: `pilgrim-manifest-${seasonId.slice(0,8)}.csv`

REPORT 2 — "Room Occupancy"
  description: "Hotel room occupancy across all accommodation"
  generate():
    const hotels = await accommodationClient.listHotels({ seasonId })
    const roomsPerHotel = await Promise.all(hotels.hotels.map(h => accommodationClient.listRooms({ hotelId: h.id })))
    headers: ["Hotel", "City", "Room No.", "Type", "Capacity", "Occupied", "Available", "Gender"]
    rows: flatten all rooms with their hotel info
    filename: `room-occupancy-${seasonId.slice(0,8)}.csv`

REPORT 3 — "Movement Manifest"
  description: "Pilgrim seat assignments for a selected movement"
  extraControls: movement selector dropdown (from movements state)
  generate():
    if (!movementId) { alert("Select a movement first"); return; }
    for each vehicle in movement: transportClient.getVehicleManifest({ movementId: movementId })
    headers: ["Vehicle Plate", "Seat No.", "Full Name", "Passport No.", "Gender", "Wheelchair"]
    rows: each seat assignment with pilgrim info
    filename: `movement-manifest-${movementId.slice(0,8)}.csv`

    Note: getVehicleManifest takes vehicleId, not movementId. First call
    transportClient.listVehicles({ movementId }) to get vehicle IDs, then loop.

REPORT 4 — "Agent Summary"
  description: "Referral agents with pilgrim counts and commission summary"
  generate():
    agents = await agentClient.listAgents({})
    headers: ["Agent Name", "Phone", "Email", "Commission Rate", "Pilgrims Referred", "Status"]
    rows: agents.agents.map(a => [
      a.name, a.phone, a.email,
      `${a.commissionRate.toFixed(2)}%`,
      String(a.pilgrimCount),
      a.isActive ? "Active" : "Inactive"
    ])
    filename: `agent-summary.csv`

CSV export helper (include in component file):
```ts
function exportCSV(filename: string, headers: string[], rows: string[][]) {
  const BOM = "﻿"; // UTF-8 BOM for Excel compatibility
  const content = [headers, ...rows]
    .map(row => row.map(v => `"${String(v ?? "").replace(/"/g, '""')}"`).join(","))
    .join("\n");
  const blob = new Blob([BOM + content], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url; a.download = filename; a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
```

Preview section (below the 2×2 grid, visible when previewRows.length > 0):
```tsx
<section style={previewCard}>
  <div style={previewHead}>
    <p style={previewTitle}>{reportLabel} — Preview (first 10 rows)</p>
    <button onClick={() => exportCSV(filename, previewHeaders, previewRows)} style={exportBtn}>
      <IconDownload size={16} /> Export CSV ({previewRows.length} rows)
    </button>
  </div>
  <div style={{ overflowX: "auto" }}>
    <table style={table}>
      <thead><tr>{previewHeaders.map(h => <th key={h} style={th}>{h}</th>)}</tr></thead>
      <tbody>{previewRows.slice(0, 10).map((row, i) => (
        <tr key={i} style={tr}>{row.map((cell, j) => <td key={j} style={td}>{cell}</td>)}</tr>
      ))}</tbody>
    </table>
  </div>
  {previewRows.length > 10 && (
    <p style={{ padding: "12px 18px", color: "var(--color-warm-400)", fontSize: 12 }}>
      … and {previewRows.length - 10} more rows in the exported file
    </p>
  )}
</section>
```

#### apps/web/app/dashboard/(shell)/reports/page.tsx

```tsx
import ReportsDashboard from "@/components/reports/ReportsDashboard";
export default function ReportsPage() { return <ReportsDashboard />; }
```

---

## NAVIGATION UPDATE

In apps/web/app/dashboard/(shell)/layout.tsx, update nav and comingSoon:

```ts
const nav = [
  ["Overview",      "/dashboard",              IconLayoutDashboard],
  ["Pilgrims",      "/dashboard/pilgrims",     IconUsers],
  ["Accommodation", "/dashboard/accommodation", IconBuildingHospital],
  ["Transport",     "/dashboard/transport",    IconBus],
  ["Products",      "/dashboard/products",     IconShoppingCart],
  ["Agents",        "/dashboard/agents",       IconUserDollar],
  ["Reports",       "/dashboard/reports",      IconFileAnalytics],
] as const;

const comingSoon = [] as const;
```

Remove the comingSoon rendering block entirely from the JSX (or render nothing if empty).
Settings stays in the bottom section as-is (already shows "Soon").

---

## DESIGN RULES FOR ALL NEW COMPONENTS

Use ONLY these CSS vars (confirmed to exist in globals.css):
  --color-emerald-50, --color-emerald-100, --color-emerald-200, --color-emerald-800, --color-emerald-900
  --color-gold-50, --color-gold-500, --color-gold-600, --color-gold-800
  --color-cream-100, --color-cream-200, --color-cream-300, --color-cream-400, --color-cream-500
  --color-warm-400, --color-warm-500, --color-warm-700, --color-warm-900
  --color-danger-600, #fdf0f0

Page header pattern (match PilgrimsDashboard exactly — NOT PageHero):
  eyebrow p: color var(--color-gold-800), fontSize 11, fontWeight 700, letterSpacing ".08em", margin "4px 0 8px"
  h1: fontSize "clamp(32px,5vw,48px)", fontWeight 500, margin 0, fontFamily Playfair Display
  Emerald button: bg var(--color-emerald-900), color var(--color-cream-100), minHeight 48
  Gold button: bg var(--color-gold-500), color var(--color-warm-900), minHeight 48

Form dialog pattern (match PilgrimFormDialog exactly):
  Sheet: display flex, flexDirection column, height 100vh, background #ffffff, borderRadius "16px 0 0 16px"
  Sticky header: position sticky, top 0, zIndex 10, background #ffffff, borderBottom "1px solid var(--color-cream-300)"
  X close button: 40×40 circle, border 1px cream-400, IconX 18px — NOT text "Close"
  Scrollable body: flex 1, overflowY auto, padding 24px
  Sticky footer: position sticky, bottom 0, background #ffffff, borderTop "1px solid var(--color-cream-300)", padding "16px 24px"
  Inputs: background #ffffff, border "1.5px solid var(--color-cream-400)", borderRadius 10, className "safrat-input"
  Field labels: 13px, fontWeight 600, color var(--color-warm-700)
  Section headers: 11px, uppercase, letterSpacing ".1em", fontWeight 700, color var(--color-warm-400)
  Field errors: 12px, color var(--color-danger-600)
  Primary button: full-width, gold, borderRadius 10

---

## EXECUTION ORDER — DO NOT SKIP STEPS

1.  Write 022_products.sql migration
2.  Write 023_agents.sql migration
3.  Apply both migrations (goose up or equivalent)
4.  Write apps/api/db/query/product.sql
5.  Write apps/api/db/query/agent.sql
6.  Run: cd apps/api && sqlc generate (fix any sqlc errors before continuing)
7.  Write proto/hajj/v1/product.proto
8.  Write proto/hajj/v1/agent.proto
9.  Run: cd proto && buf generate (fix any buf errors before continuing)
10. Write apps/api/internal/domain/product.go
11. Write apps/api/internal/domain/agent.go
12. Write apps/api/internal/repository/product.go
13. Write apps/api/internal/repository/agent.go
14. Write apps/api/internal/service/product.go
15. Write apps/api/internal/service/agent.go
16. Write apps/api/internal/handler/product.go
17. Write apps/api/internal/handler/agent.go
18. Update apps/api/cmd/server/main.go (wire both)
19. Run: cd apps/api && go build ./... (fix all Go errors before frontend)
20. Update apps/web/lib/rpc.ts (add productClient and agentClient)
21. Write apps/web/components/products/ProductsDashboard.tsx
22. Write apps/web/components/products/ProductFormDialog.tsx
23. Write apps/web/app/dashboard/(shell)/products/page.tsx
24. Write apps/web/components/agents/AgentsDashboard.tsx
25. Write apps/web/components/agents/AgentFormDialog.tsx
26. Write apps/web/app/dashboard/(shell)/agents/page.tsx
27. Write apps/web/components/reports/ReportsDashboard.tsx
28. Write apps/web/app/dashboard/(shell)/reports/page.tsx
29. Update apps/web/app/dashboard/(shell)/layout.tsx nav
30. Run: pnpm --filter web build (fix all TypeScript errors)

---

## VERIFICATION CHECKLIST

After all steps complete, verify each item:

Backend:
  [ ] cd apps/api && go build ./... → zero errors
  [ ] Products table exists in DB with correct columns
  [ ] Agents table exists in DB with correct columns
  [ ] pilgrims.agent_id column exists
  [ ] sqlc generated: internal/gen/db/product.sql.go, agent.sql.go
  [ ] buf generated: proto-gen/hajj/v1/product_connect.ts, product_pb.ts, agent_connect.ts, agent_pb.ts
  [ ] hajjv1connect.NewProductServiceHandler exists in generated code
  [ ] hajjv1connect.NewAgentServiceHandler exists in generated code

Frontend:
  [ ] pnpm --filter web build → zero TypeScript errors
  [ ] grep -r "var(--border-default)\|var(--bg-input)\|var(--text-secondary)\|var(--action-primary-bg)" apps/web/components/products apps/web/components/agents apps/web/components/reports → zero matches
  [ ] Nav shows Products, Agents, Reports without "Soon" badge
  [ ] /dashboard/products loads without error
  [ ] /dashboard/agents loads without error
  [ ] /dashboard/reports loads without error
  [ ] All 4 report generate buttons are functional
  [ ] CSV download works (file opens in Excel with correct columns)
