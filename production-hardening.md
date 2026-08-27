# Production Hardening — Dockerfile + CI/CD

> **Constraint:** Do NOT edit any existing files not explicitly listed. Follow the Go 3-layer
> architecture (handler → service → repository). Never skip layers.

---

## Context

`DEPLOY.md` already exists and is the authoritative deployment reference. Three files still need
to be created to make the deployment pipeline operational:

1. `apps/api/Dockerfile` — multi-stage Go build (produces both `api` and `worker` binaries)
2. `apps/web/Dockerfile` — multi-stage Next.js standalone build
3. `.github/workflows/deploy.yml` — GitHub Actions CI + build + VPS deploy

One prerequisite: `apps/web/next.config.ts` (or `.js`) must have `output: 'standalone'` — the
web Dockerfile copies `apps/web/.next/standalone` and will fail if standalone mode is not enabled.
Check the file before creating the Dockerfile; add the key if missing.

---

## Task 1 — Check and update `apps/web/next.config.ts`

Read `apps/web/next.config.ts`. If it does NOT already have `output: 'standalone'`, add it inside
the `nextConfig` object:

```ts
const nextConfig: NextConfig = {
  output: 'standalone',
  // ... rest of existing config, unchanged
};
```

Do not remove or change any other config keys. If `output: 'standalone'` is already present, skip
this task entirely.

---

## Task 2 — Create `apps/api/Dockerfile`

Create the file at `apps/api/Dockerfile` with this exact content (no modifications):

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./cmd/worker

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /api .
COPY --from=builder /worker .
USER appuser
EXPOSE 8080
CMD ["./api"]
```

Notes:
- Go version is `1.25` (matches `go.mod` — verify `go version` in `apps/api/go.mod`; if it says
  `go 1.22` or another version, use that instead of 1.25).
- Both binaries (`api` and `worker`) ship in one image. The `worker` compose service reuses this
  same image with `command: ["./worker"]` in `docker-compose.prod.yml`. The worker must run — it
  handles SOS escalation every 1 minute and agent tier recalculation every 5 minutes.
- `ca-certificates` is required for outbound TLS (Firebase push, Sentry, Xendit webhooks).
- `tzdata` is required for correct time zone handling in scheduled worker tasks.

---

## Task 3 — Create `apps/web/Dockerfile`

Create the file at `apps/web/Dockerfile` with this exact content:

```dockerfile
FROM node:20-alpine AS base
RUN corepack enable && corepack prepare pnpm@9 --activate

FROM base AS deps
WORKDIR /app
COPY package.json pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/ packages/
RUN pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# NEXT_PUBLIC_* must be baked at build time — they are inlined into the
# client bundle. Anything read by a client component must be listed here
# or it silently becomes undefined in the browser.
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_APP_URL
ARG NEXT_PUBLIC_VAPID_PUBLIC_KEY
ARG NEXT_PUBLIC_SENTRY_DSN
ARG NEXT_PUBLIC_FIREBASE_API_KEY
ARG NEXT_PUBLIC_FIREBASE_PROJECT_ID
ARG NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID
ARG NEXT_PUBLIC_FIREBASE_APP_ID
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL
ENV NEXT_PUBLIC_VAPID_PUBLIC_KEY=$NEXT_PUBLIC_VAPID_PUBLIC_KEY
ENV NEXT_PUBLIC_SENTRY_DSN=$NEXT_PUBLIC_SENTRY_DSN
ENV NEXT_PUBLIC_FIREBASE_API_KEY=$NEXT_PUBLIC_FIREBASE_API_KEY
ENV NEXT_PUBLIC_FIREBASE_PROJECT_ID=$NEXT_PUBLIC_FIREBASE_PROJECT_ID
ENV NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=$NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID
ENV NEXT_PUBLIC_FIREBASE_APP_ID=$NEXT_PUBLIC_FIREBASE_APP_ID

RUN pnpm --filter @hajj-saas/web build

FROM base AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/apps/web/.next/standalone ./
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public
USER appuser
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
```

Notes:
- Context is the monorepo root (`.`), not `apps/web` — the build needs `packages/` for proto-gen
  and UI tokens. The CI step uses `file: apps/web/Dockerfile` with `context: .`.
- `pnpm --filter @hajj-saas/web build` — verify the web package name matches `apps/web/package.json`
  `"name"` field. If the package name is different (e.g. `web` or `safrat-web`), use that name
  instead.
- `node apps/web/server.js` — this is the Next.js standalone server. Path is relative to the
  standalone output directory which is the WORKDIR.

---

## Task 4 — Create `.github/workflows/deploy.yml`

Create the directory `.github/workflows/` if it does not exist. Create the file
`.github/workflows/deploy.yml`:

```yaml
name: Deploy to VPS

on:
  push:
    branches: [main]

env:
  API_IMAGE: ghcr.io/${{ github.repository_owner }}/safrat-api
  WEB_IMAGE: ghcr.io/${{ github.repository_owner }}/safrat-web

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: apps/api/go.mod

      - name: Go test
        run: cd apps/api && go test ./...

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install pnpm
        run: corepack enable && corepack prepare pnpm@9 --activate

      - name: Install dependencies
        run: pnpm install --frozen-lockfile

      - name: TypeScript check
        run: pnpm --filter @hajj-saas/web typecheck

  build-and-deploy:
    needs: test
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push API image
        uses: docker/build-push-action@v5
        with:
          context: apps/api
          push: true
          cache-from: type=gha
          cache-to: type=gha,mode=max
          tags: |
            ${{ env.API_IMAGE }}:${{ github.sha }}
            ${{ env.API_IMAGE }}:latest

      - name: Build and push Web image
        uses: docker/build-push-action@v5
        with:
          context: .
          file: apps/web/Dockerfile
          push: true
          cache-from: type=gha
          cache-to: type=gha,mode=max
          tags: |
            ${{ env.WEB_IMAGE }}:${{ github.sha }}
            ${{ env.WEB_IMAGE }}:latest
          build-args: |
            NEXT_PUBLIC_API_URL=https://api.safrat.com
            NEXT_PUBLIC_APP_URL=https://app.safrat.com
            NEXT_PUBLIC_VAPID_PUBLIC_KEY=${{ secrets.NEXT_PUBLIC_VAPID_PUBLIC_KEY }}
            NEXT_PUBLIC_SENTRY_DSN=${{ secrets.NEXT_PUBLIC_SENTRY_DSN }}
            NEXT_PUBLIC_FIREBASE_API_KEY=${{ secrets.NEXT_PUBLIC_FIREBASE_API_KEY }}
            NEXT_PUBLIC_FIREBASE_PROJECT_ID=${{ secrets.NEXT_PUBLIC_FIREBASE_PROJECT_ID }}
            NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=${{ secrets.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID }}
            NEXT_PUBLIC_FIREBASE_APP_ID=${{ secrets.NEXT_PUBLIC_FIREBASE_APP_ID }}

      - name: Deploy to VPS
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.VPS_HOST }}
          username: ${{ secrets.VPS_USER }}
          key: ${{ secrets.VPS_SSH_KEY }}
          script: |
            set -e
            cd /home/deploy/safrat

            # Pull latest repo (migration files live in repo)
            git pull origin main

            # Pull new images
            docker compose -f docker-compose.prod.yml --env-file .env.prod pull

            # Run goose migrations before restarting API
            source .env.prod
            NETWORK=$(docker network ls --filter name=safrat_internal --format '{{.Name}}' | head -1)
            docker run --rm \
              --network "${NETWORK:-safrat_internal}" \
              -v "$(pwd)/apps/api/db/migrations:/migrations" \
              ghcr.io/kukymbr/goose-docker:latest \
              goose -dir /migrations postgres \
              "postgresql://safrat:${POSTGRES_PASSWORD}@postgres:5432/safrat" up

            # Rolling restart — API first (no downtime for pure API changes),
            # web second. Worker restarts together with API (same image).
            docker compose -f docker-compose.prod.yml --env-file .env.prod \
              up -d --no-deps api worker
            sleep 5
            docker compose -f docker-compose.prod.yml --env-file .env.prod \
              up -d --no-deps web

            # Prune old images to free disk space
            docker image prune -f
```

**GitHub Secrets to configure (Settings → Secrets → Actions):**

| Secret | Where to get it |
|---|---|
| `VPS_HOST` | VPS IP address (e.g. `123.45.67.89`) |
| `VPS_USER` | `deploy` (the user created in DEPLOY.md §2) |
| `VPS_SSH_KEY` | Private key pair for the deploy user (`ssh-keygen -t ed25519`) |
| `NEXT_PUBLIC_VAPID_PUBLIC_KEY` | Firebase Cloud Messaging → Web Push certificates |
| `NEXT_PUBLIC_SENTRY_DSN` | Sentry project DSN (client-side) |
| `NEXT_PUBLIC_FIREBASE_API_KEY` | Firebase project settings → General |
| `NEXT_PUBLIC_FIREBASE_PROJECT_ID` | Firebase project settings → General |
| `NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID` | Firebase project settings → General |
| `NEXT_PUBLIC_FIREBASE_APP_ID` | Firebase project settings → General |

> `GITHUB_TOKEN` is automatically provided by GitHub Actions — no manual secret needed for GHCR push.

---

## Verification

After all four tasks:

1. Run `docker build -t safrat-api-test apps/api/` locally — must complete without error.
2. Run `docker build -t safrat-web-test -f apps/web/Dockerfile .` from monorepo root — must
   complete without error (can pass `--build-arg NEXT_PUBLIC_API_URL=http://localhost:8080`
   for local test).
3. Confirm `.github/workflows/deploy.yml` exists and the YAML is valid (run
   `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/deploy.yml'))"` or use
   `actionlint` if available).
4. Confirm `apps/web/next.config.ts` has `output: 'standalone'`.
