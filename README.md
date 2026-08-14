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
```

## Local services

Copy `.env.example` to `.env`, set `BETTER_AUTH_SECRET`, then start PostgreSQL and apply migrations:

```bash
docker compose up -d postgres
goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up
```

The Go API fails closed unless `DATABASE_URL`, `BETTER_AUTH_SECRET`, and `CORS_ALLOWED_ORIGIN` are configured.

## Deployment

`apps/api/Dockerfile` builds a minimal non-root API image. Before deploying, run `pnpm buf:generate`, `cd apps/api && sqlc generate`, and apply Goose migrations with the direct database URL as a separate release step.
