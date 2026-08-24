# Hajj & Umrah Operator SaaS

The implementation follows `CODEX_SPEC.md`. The first milestone is operator onboarding: a Better Auth organization maps to an operator, then the operator creates and activates a season.

## Prerequisites

- Bun 1.x for local development (Node.js 20 LTS in production)
- pnpm 9.x
- Go 1.22+
- Buf CLI and sqlc for generated API/database code
- PostgreSQL 16 (self-hosted in Docker for local and deployed environments)

## Commands

```bash
pnpm install
pnpm buf:generate
pnpm dev

cd apps/api
go run ./cmd/server

# Agent tier-recalculation worker (needs Redis, see below)
go run ./cmd/worker
```

## Local services

Copy `.env.example` to `.env`, set `BETTER_AUTH_SECRET`, then start PostgreSQL and Redis and apply migrations:

```bash
docker compose up -d postgres redis minio minio-init
goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up
```

For the storefront CMS, export the `S3_*` values from `.env.example` before
starting the API. MinIO runs at `http://localhost:9000`, its console is at
`http://localhost:9001`, and the init container creates the public-read
`safrat-uploads` bucket. Production media should use a dedicated S3-compatible
bucket and public asset domain, not these local credentials.

The Go API fails closed unless `DATABASE_URL`, `BETTER_AUTH_SECRET`, and `CORS_ALLOWED_ORIGIN` are configured. `cmd/worker` additionally requires `REDIS_URL` and periodically recalculates agent tiers (Bronze/Silver/Gold) from referred pilgrim counts — it does not compute payouts, which need order data that doesn't exist yet.

## Deployment

`apps/api/Dockerfile` builds a minimal non-root API image. Before deploying, run `pnpm buf:generate`, `cd apps/api && sqlc generate`, and apply Goose migrations with the direct database URL as a separate release step.
