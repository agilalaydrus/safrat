# Safrat — VPS Deployment Guide

> **Stack:** Ubuntu 22.04 LTS · Docker Compose · nginx · Let's Encrypt · GitHub Actions
>
> **Port map (host-facing, bound to 127.0.0.1 only):**
> - `9100` → Go API (container internal: 8080)
> - `9101` → Next.js Web (container internal: 3000)
> - `5432` NOT exposed — PostgreSQL stays inside Docker network only
> - `6379` NOT exposed — Redis stays inside Docker network only (§4). The
>   `worker` reads it for the asynq job queue; the `api` reads it too when
>   `REDIS_URL` is set, for the monitoring event bus, shared operator cache
>   invalidation, and distributed public-endpoint rate limiter. Keep it set
>   whenever more than one `api` replica is running.
> - `worker` (§4, `cmd/worker`) has no host port at all — it's a
>   background scheduler, not an HTTP service

---

## 1. VPS Initial Setup

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y git curl ufw fail2ban nginx certbot python3-certbot-nginx

# Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker

# Firewall — SSH + HTTP + HTTPS only
sudo ufw allow OpenSSH
sudo ufw allow 'Nginx Full'
sudo ufw enable
sudo ufw status
```

---

## 2. Deploy User

```bash
sudo adduser deploy
sudo usermod -aG docker deploy
sudo usermod -aG sudo deploy
su - deploy

mkdir -p ~/.ssh
echo "YOUR_GITHUB_ACTIONS_PUBLIC_KEY" >> ~/.ssh/authorized_keys
chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys
```

---

## 3. Project Directory

```bash
mkdir -p /home/deploy/safrat
cd /home/deploy/safrat
git clone https://github.com/YOUR_ORG/hajj-saas.git .
```

---

## 4. Production Docker Compose

`/home/deploy/safrat/docker-compose.prod.yml`:

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    restart: always
    environment:
      POSTGRES_USER: safrat
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: safrat
    volumes:
      - postgres_data:/var/lib/postgresql/data
    # No ports exposed — internal network only
    networks:
      - internal

  api:
    image: ghcr.io/YOUR_ORG/safrat-api:${IMAGE_TAG:-latest}
    restart: always
    environment:
      # No DATABASE_URL — pgxpool.New(ctx, "") in main.go resolves these
      # directly, avoiding URL parsing entirely (a raw password embedded in
      # a URL broke this in production: pgx silently produced an empty
      # host and fell back to a unix socket instead of erroring).
      PGHOST: postgres
      PGPORT: "5432"
      PGUSER: safrat
      PGPASSWORD: ${POSTGRES_PASSWORD}
      PGDATABASE: safrat
      BETTER_AUTH_SECRET: ${BETTER_AUTH_SECRET}
      CORS_ALLOWED_ORIGIN: https://tawafiqhub.id
      SENTRY_DSN: ${SENTRY_DSN}
      FIREBASE_SERVICE_ACCOUNT_JSON: ${FIREBASE_SERVICE_ACCOUNT_JSON}
      XENDIT_SECRET_KEY: ${XENDIT_SECRET_KEY}
      XENDIT_WEBHOOK_TOKEN: ${XENDIT_WEBHOOK_TOKEN}
      XENDIT_WEBHOOK_ALLOWED_IPS: ${XENDIT_WEBHOOK_ALLOWED_IPS}
    ports:
      - "127.0.0.1:9100:8080"   # nginx → localhost:9100 → container :8080
    depends_on:
      - postgres
    networks:
      - internal

  # Same image as api, different entrypoint — see the note in §6. Without
  # this service, SOS alerts never escalate past ACTIVE to a coordinator.
  worker:
    image: ghcr.io/YOUR_ORG/safrat-api:${IMAGE_TAG:-latest}
    command: ["./worker"]
    restart: always
    environment:
      PGHOST: postgres
      PGPORT: "5432"
      PGUSER: safrat
      PGPASSWORD: ${POSTGRES_PASSWORD}
      PGDATABASE: safrat
      REDIS_URL: redis://redis:6379
      SENTRY_DSN: ${SENTRY_DSN}
      FIREBASE_SERVICE_ACCOUNT_JSON: ${FIREBASE_SERVICE_ACCOUNT_JSON}
    depends_on:
      - postgres
      - redis
    networks:
      - internal

  redis:
    image: redis:7-alpine
    restart: always
    # No ports exposed — internal network only, same as postgres
    networks:
      - internal

  web:
    image: ghcr.io/YOUR_ORG/safrat-web:${IMAGE_TAG:-latest}
    restart: always
    environment:
      # apps/web/lib/auth.ts's Pool() reads these directly (no DATABASE_URL
      # string) — a raw password embedded in a URL breaks Node's strict URL
      # parser if it contains certain characters (this broke in production).
      PGHOST: postgres
      PGPORT: "5432"
      PGUSER: safrat
      PGPASSWORD: ${POSTGRES_PASSWORD}
      PGDATABASE: safrat
      BETTER_AUTH_SECRET: ${BETTER_AUTH_SECRET}
      BETTER_AUTH_URL: https://tawafiqhub.id
      GOOGLE_CLIENT_ID: ${GOOGLE_CLIENT_ID}
      GOOGLE_CLIENT_SECRET: ${GOOGLE_CLIENT_SECRET}
      RESEND_API_KEY: ${RESEND_API_KEY}
      RESEND_FROM_EMAIL: ${RESEND_FROM_EMAIL}
      SENTRY_DSN: ${SENTRY_DSN}
    ports:
      - "127.0.0.1:9101:3000"   # nginx → localhost:9101 → container :3000
    depends_on:
      - postgres
    networks:
      - internal

volumes:
  postgres_data:

networks:
  internal:
    driver: bridge
```

> **NEXT_PUBLIC_* warning:** Next.js bakes these at build time, not runtime.
> Pass them as Docker build args in CI — see Section 8.

`/home/deploy/safrat/.env.prod` (never commit):

```bash
POSTGRES_PASSWORD=use_a_strong_random_password

# Better Auth — generate with: openssl rand -base64 32
# Must be identical in both api and web services
BETTER_AUTH_SECRET=your_secret_here

# Google Sign-In (Better Auth social provider) — web service only, never
# reaches the Go API directly. Redirect URI: https://tawafiqhub.id/api/auth/callback/google
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...

# Redis — self-hosted via the `redis` compose service above, not Upstash;
# cmd/worker always reads REDIS_URL (asynq queue); the api also reads it for
# monitoring pub/sub, operator-cache invalidation, and distributed rate limits.
# REDIS_URL is hardcoded to redis://redis:6379 in the worker service block
# above since it's an internal-network hostname, not a secret — nothing
# needed here.

# Firebase (push notifications) — optional, no-op on both api and web when
# unset. Server-only JSON, not the split PROJECT_ID/CLIENT_EMAIL/PRIVATE_KEY
# vars an earlier version of this doc had — Project Settings > Service
# Accounts > Generate new private key, paste the whole file as one line.
FIREBASE_SERVICE_ACCOUNT_JSON='{"type":"service_account",...}'
# Client-safe web config + Web Push key — Project Settings > General / Cloud Messaging.
# Also passed as Docker build args (§6/§8) since they're baked into the
# client bundle, not just read at runtime.
NEXT_PUBLIC_FIREBASE_API_KEY=...
NEXT_PUBLIC_FIREBASE_PROJECT_ID=...
NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=...
NEXT_PUBLIC_FIREBASE_APP_ID=...
NEXT_PUBLIC_VAPID_PUBLIC_KEY=...

# Email — password reset + email verification, both link-based
# (apps/web/lib/email.ts). No-op (logged) when unset. RESEND_FROM_EMAIL
# must belong to a domain verified in Resend, or falls back to
# onboarding@resend.dev (sandbox — only deliverable to the Resend
# account's own address).
RESEND_API_KEY=re_...
RESEND_FROM_EMAIL=noreply@tawafiqhub.id

# Xendit (Module 7 — Orders & Payments) — apps/api only, unrelated to
# email/web. Without XENDIT_SECRET_KEY, checkout fails fast with a clear
# error (internal/payment/xendit.go) rather than creating an unpayable
# order. Register the webhook URL in the Xendit Dashboard > Settings >
# Webhooks: https://api.tawafiqhub.id/webhooks/xendit — XENDIT_WEBHOOK_TOKEN
# must match the "Verification Token" shown on that same page.
XENDIT_SECRET_KEY=xnd_production_...
XENDIT_WEBHOOK_TOKEN=...

# Observability — optional, no-op on both api and web when unset
SENTRY_DSN=...
NEXT_PUBLIC_SENTRY_DSN=...

# Self-hosted MinIO storefront media. Keep the root credentials exclusive to
# minio-init; the API receives the separate bucket-scoped S3 service user.
MINIO_ROOT_USER=safrat-root
MINIO_ROOT_PASSWORD=<openssl-rand-hex-32>
S3_ENDPOINT=https://assets.tawafiqhub.id
S3_REGION=us-east-1
S3_BUCKET=safrat-uploads
S3_ACCESS_KEY_ID=safrat-storefront
S3_SECRET_ACCESS_KEY=<openssl-rand-hex-32>
S3_PUBLIC_BASE_URL=https://assets.tawafiqhub.id/safrat-uploads
S3_FORCE_PATH_STYLE=true
```

`minio` stores data in the persistent `minio_data` Docker volume and publishes
only its S3 API on loopback port 9102. The administrative console is disabled
and port 9001 is not exposed. Nginx terminates wildcard TLS at
`assets.tawafiqhub.id`; the API container maps that hostname to the Docker host
so server-side validation and promotion do not hairpin through the public
network.

The service user's policy grants `s3:ListBucket` only for the `storefront/`
prefix — needed by the one-time `/storefront-backfill` job, which cannot
enumerate existing media without it. Note that MinIO's root credentials bypass
this entirely, so any local test run as root will not exercise it.

The one-shot `minio-init` service is idempotent and runs on every deployment.
It creates the bucket, rotates/reconciles the least-privilege service user,
limits anonymous reads to `storefront/`, and expires only
`storefront-pending/` objects after one day. MinIO
Community configures CORS cluster-wide rather than per bucket; the Compose
service pins `MINIO_API_CORS_ALLOW_ORIGIN` to `https://tawafiqhub.id`. This
instance hosts only TawafiqHub's bucket. Do not grant public access to the whole
bucket and never expose either secret through a `NEXT_PUBLIC_*` variable.

The API signs both `Content-Type: image/webp` and the optimized byte length,
uses random tenant-scoped object keys, and expires upload URLs after 10 minutes.
Uploads first enter `storefront-pending/`; only a fully decoded and validated
WebP is copied to the durable `storefront/` prefix.

```bash
cd /home/deploy/safrat
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d minio
docker compose -f docker-compose.prod.yml --env-file .env.prod up --no-deps minio-init
docker compose -f docker-compose.prod.yml --env-file .env.prod ps minio
curl --fail https://assets.tawafiqhub.id/minio/health/live
```

The wildcard DNS record already covers `assets.tawafiqhub.id`, and the existing
apex-plus-wildcard certificate covers its HTTPS endpoint. Keep VPS snapshots
enabled. A snapshot or backup stored on the same VPS is not a disaster-recovery
copy; add an offsite target before media becomes business-critical.

Start services:

```bash
cd /home/deploy/safrat
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs api --tail=50
```

---

## 5. Database Migrations

Two sets of migrations — run both on first deploy and after every release:

```bash
cd /home/deploy/safrat
source .env.prod

# Step 1 — Better Auth migrations (user, session, account, organization, etc.)
# Must run BEFORE goose: 025_fix_groups_leader_id.sql and later reference
# "user", which only exists after this runs. `better-auth` itself has no
# CLI — the real package is `@better-auth/cli`, and it needs python3/make/g++
# to build its native `better-sqlite3` dependency even though we use Postgres.
# Uses PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE, same as apps/web/lib/
# auth.ts's Pool() at runtime — not a DATABASE_URL string, which broke in
# production ("TypeError: Invalid URL") whenever the real password hit
# pg-connection-string's strict parser.
docker run --rm \
  --network safrat_internal \
  -v $(pwd):/repo \
  -w /repo \
  -e PGHOST=postgres \
  -e PGPORT=5432 \
  -e PGUSER=safrat \
  -e PGPASSWORD="${POSTGRES_PASSWORD}" \
  -e PGDATABASE=safrat \
  node:20-alpine \
  sh -c 'apk add --no-cache python3 make g++ && corepack enable && corepack prepare pnpm@9 --activate && pnpm install --frozen-lockfile --config.node-linker=hoisted && cd apps/web && npx @better-auth/cli@1.4.21 migrate --yes'

# Step 2 — goose migrations (business schema: operators, pilgrims, seasons, etc.)
docker run --rm \
  --network safrat_internal \
  -v $(pwd)/apps/api/db/migrations:/migrations \
  -e GOOSE_DRIVER=postgres \
  -e GOOSE_DBSTRING="host=postgres port=5432 user=safrat dbname=safrat sslmode=disable" \
  -e GOOSE_MIGRATION_DIR=/migrations \
  -e PGPASSWORD="${POSTGRES_PASSWORD}" \
  ghcr.io/kukymbr/goose-docker:latest \
  up

# Check migration status
docker run --rm \
  --network safrat_internal \
  -v $(pwd)/apps/api/db/migrations:/migrations \
  -e GOOSE_DRIVER=postgres \
  -e GOOSE_DBSTRING="host=postgres port=5432 user=safrat dbname=safrat sslmode=disable" \
  -e GOOSE_MIGRATION_DIR=/migrations \
  -e PGPASSWORD="${POSTGRES_PASSWORD}" \
  ghcr.io/kukymbr/goose-docker:latest \
  status
```

> Network name is `<project-dir>_internal`. Verify with: `docker network ls`

---

## 6. Dockerfiles

### apps/api/Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./cmd/worker

FROM alpine:3.19
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /api .
COPY --from=builder /worker .
USER appuser
EXPOSE 8080
CMD ["./api"]
```

> Pin the Go version here to match `go.mod`. Check with `go version`.
>
> **Both binaries ship in one image** — `cmd/server` (the API) and
> `cmd/worker` (the asynq scheduler: agent tier recalculation every 5min,
> **SOS escalation every 1min** — see CLAUDE.md) are built together so the
> `worker` compose service below can reuse this same image with
> `command: ["./worker"]` instead of needing a second CI build/push step.
> Skipping the worker service entirely means SOS alerts never escalate to
> coordinators in production — this is not optional infrastructure.

### apps/web/Dockerfile

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

# NEXT_PUBLIC_* must be baked at build time — anything read by a client
# component (not just the server-side firebase-messaging-sw.js route
# handler, which reads process.env live at request time instead) needs to
# be listed here or it silently ends up undefined in the browser bundle.
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

---

## 7. nginx Configuration

The production source of truth is `deploy/nginx/safrat.conf`. Install the
root-owned helper once and grant the deploy user passwordless access to that
single command only:

```bash
cd /home/deploy/safrat
sudo install -o root -g root -m 0755 \
  deploy/nginx/install-nginx \
  /usr/local/sbin/safrat-install-nginx

echo 'deploy ALL=(root) NOPASSWD: /usr/local/sbin/safrat-install-nginx' \
  | sudo tee /etc/sudoers.d/safrat-nginx >/dev/null
sudo chmod 0440 /etc/sudoers.d/safrat-nginx
sudo visudo -cf /etc/sudoers.d/safrat-nginx
sudo -u deploy sudo -n /usr/local/sbin/safrat-install-nginx
```

The workflow invokes the helper only after database migrations succeed. The
helper promotes the combined config to the VPS's active
`/etc/nginx/sites-available/tawafiqhub` target and replaces the legacy
`tawafiqhub-root` config with a tracked neutral file. It verifies both enabled
symlinks before writing, validates with `nginx -t`, and restores both previous
files on any promotion, validation, or reload failure. Post-deploy smoke tests
also cover wildcard TLS/HTTP routing, reserved hosts, tenant CORS, and the
canonical apex redirects before the workflow can turn green.

### Wildcard tenant DNS and TLS

`deploy/nginx/safrat.conf` serves every first-level operator hostname through
the web container, while exact `api`, `app`, and `www` blocks retain priority.
Complete these prerequisites **before pushing the wildcard nginx config to
`main`**, otherwise nginx validation will intentionally stop the deployment:

First, preflight existing production data. This query must return no rows;
migration 080 deliberately fails rather than silently rename a live public URL:

```bash
cd /home/deploy/safrat
docker compose -f docker-compose.prod.yml --env-file .env.prod exec -T postgres \
  psql -U safrat -d safrat -c "
    SELECT slug FROM operators
    WHERE slug IS NOT NULL AND (
      length(slug) NOT BETWEEN 3 AND 63 OR
      slug !~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$' OR
      slug IN ('admin','api','app','auth','dashboard','docs','help','status','support','www')
    );"
```

1. In Hostinger DNS, add an `A` record with name `*`, value
   `103.179.66.25`, and TTL `300` during rollout (`3600` afterward is fine).
   Do not delete the existing apex, `www`, or `api` records. Verify from two
   public resolvers:

   ```bash
   dig +short tenant-probe.tawafiqhub.id A @1.1.1.1
   dig +short tenant-probe.tawafiqhub.id A @8.8.8.8
   ```

   Both commands must return `103.179.66.25`.

2. Make the `deploy/tls` files available from the reviewed non-production
   branch in a separate worktree. This leaves `/home/deploy/safrat` on the
   currently deployed `main` commit while TLS is bootstrapped:

   ```bash
   sudo -u deploy git -C /home/deploy/safrat fetch origin codex/wildcard-tenants
   sudo -u deploy git -C /home/deploy/safrat worktree add \
     /home/deploy/safrat-wildcard \
     origin/codex/wildcard-tenants
   sudo /home/deploy/safrat-wildcard/deploy/tls/install-wildcard-tls \
     halo@tawafiqhub.id
   ```

   The prompt reads the Hostinger API token without echoing it. The installer
   downloads pinned `lego` v5.3.1, verifies the published SHA-256 checksum,
   stores the token at `/etc/safrat/secrets/hostinger-api-token` with mode
   `0600`, obtains one EC certificate for `tawafiqhub.id` plus
   `*.tawafiqhub.id`, and enables a daily systemd renewal check. To rotate the
   token later, rerun with `--replace-token`. Never place this token in
   `.env.prod`, GitHub Secrets used by the application, or the repository;
   Hostinger tokens currently carry broad account permissions.

3. Verify the certificate and automated renewal unit:

   ```bash
   sudo openssl x509 \
     -in /etc/safrat/lego/certificates/tawafiqhub.id.crt \
     -noout -subject -issuer -dates -ext subjectAltName
   sudo systemctl status safrat-wildcard-tls.timer --no-pager
   sudo systemctl start safrat-wildcard-tls.service
   sudo journalctl -u safrat-wildcard-tls.service -n 100 --no-pager
   ```

   The renewal hook validates expiry, both SAN entries, and the certificate/key
   pair before `nginx -t` and reload. Issuance uses a fixed 120-second DNS wait
   before Let's Encrypt validation, avoiding a known class of false-negative
   local/authoritative prechecks after Hostinger and public resolvers already
   agree on both TXT values. The existing Certbot timer can remain while its
   legacy exact-host certificates are still referenced.

4. Reinstall the root-owned nginx promotion helper because the wildcard release
   adds certificate preconditions:

   ```bash
   sudo install -o root -g root -m 0755 \
     /home/deploy/safrat-wildcard/deploy/nginx/install-nginx \
     /usr/local/sbin/safrat-install-nginx
   sudo -u deploy sudo -n /usr/local/sbin/safrat-install-nginx
   ```

   The helper refuses promotion if the wildcard certificate is missing, checks
   both active historical symlinks, and restores both prior configs if
   installation, `nginx -t`, or reload fails.

5. Only after DNS and TLS pass, push/deploy `main`. The workflow then requires:
   API/apex/service-worker success; canonical redirects; wildcard HTTP→HTTPS;
   wildcard TLS reaching the Next.js 404 for an unknown tenant; direct Nginx
   rejection of `admin`; and tenant-origin CORS.

Useful production checks:

```bash
curl -I http://tenant-probe.tawafiqhub.id/probe
curl -I https://tenant-probe.tawafiqhub.id/
curl -I https://admin.tawafiqhub.id/
sudo nginx -t
sudo systemctl is-active nginx safrat-wildcard-tls.timer
```

---

## 8. GitHub Actions CI/CD

**Current state:** `.github/workflows/deploy.yml` is live — `test` (Go test + TS typecheck) then
`build-and-deploy` (builds + pushes both images to GHCR, then SSHes into the VPS to pull, run
goose migrations, and rolling-restart `api`/`worker` then `web`). It runs on every push to `main`,
and can also be triggered manually (`workflow_dispatch`) with a `skip_build` input — set to `y` to
redeploy the existing `:latest` images without rebuilding (e.g. to retry a failed deploy step).
Read the workflow file itself for the exact script — this doc no longer keeps an inline copy, to
avoid the two drifting apart.

**GitHub Secrets required:**

| Secret | Value |
|---|---|
| `VPS_HOST` | VPS IP or hostname |
| `VPS_USER` | `deploy` |
| `VPS_SSH_KEY` | Private key for deploy user |
| `NEXT_PUBLIC_VAPID_PUBLIC_KEY` | VAPID public key |
| `NEXT_PUBLIC_SENTRY_DSN` | Sentry client DSN |
| `NEXT_PUBLIC_FIREBASE_API_KEY` | Firebase web config |
| `NEXT_PUBLIC_FIREBASE_PROJECT_ID` | Firebase web config |
| `NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID` | Firebase web config |
| `NEXT_PUBLIC_FIREBASE_APP_ID` | Firebase web config |

---

## 9. Database Backup (Daily)

```bash
cat > /home/deploy/backup-db.sh << 'EOF'
#!/bin/bash
set -e
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/home/deploy/backups"
mkdir -p "$BACKUP_DIR"

docker compose -f /home/deploy/safrat/docker-compose.prod.yml \
  exec -T postgres \
  pg_dump -U safrat safrat | gzip > "${BACKUP_DIR}/safrat_${DATE}.sql.gz"

find "$BACKUP_DIR" -name "*.sql.gz" -mtime +7 -delete
echo "[$(date)] Backup done"
EOF

chmod +x /home/deploy/backup-db.sh
/home/deploy/backup-db.sh  # test it

# Schedule daily 02:00
crontab -e
# Add: 0 2 * * * /home/deploy/backup-db.sh >> /home/deploy/backup.log 2>&1
```

---

## 10. Generate BETTER_AUTH_SECRET

```bash
# Generate a secure secret (run once, save to .env.prod and GitHub Secrets)
openssl rand -base64 32

# This SAME value must be in:
# - .env.prod → BETTER_AUTH_SECRET (used by both api and web containers)
# - apps/web .env.local → BETTER_AUTH_SECRET (for local dev)
# - apps/api .env → BETTER_AUTH_SECRET (for local dev)
```

---

## 11. Useful Commands

```bash
# All logs
docker compose -f docker-compose.prod.yml logs -f

# API logs
docker compose -f docker-compose.prod.yml logs api -f --tail=100

# Worker logs — check this if SOS alerts aren't escalating past ACTIVE
docker compose -f docker-compose.prod.yml logs worker -f --tail=100

# Restart one service
docker compose -f docker-compose.prod.yml restart api

# Connect to PostgreSQL
docker compose -f docker-compose.prod.yml exec postgres psql -U safrat -d safrat

# nginx
sudo nginx -t && sudo systemctl reload nginx

# Wildcard cert expiry + renewal timer (Certbot still owns legacy exact hosts)
sudo openssl x509 -in /etc/safrat/lego/certificates/tawafiqhub.id.crt -noout -dates
sudo systemctl status safrat-wildcard-tls.timer --no-pager

# Verify ports are NOT exposed externally (must show 127.0.0.1, never 0.0.0.0)
sudo ss -tlnp | grep -E '9100|9101|5432'
```

---

## 12. Quick Migration: Local Dev DB → VPS (First Deploy)

```bash
# 1. Dump local dev DB
docker compose exec postgres pg_dump -U safrat safrat > dev_dump.sql

# 2. Copy to VPS
scp dev_dump.sql deploy@YOUR_VPS_IP:/home/deploy/

# 3. On VPS — restore
cd /home/deploy/safrat
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d postgres
sleep 5

cat /home/deploy/dev_dump.sql | \
  docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U safrat -d safrat

# 4. Verify
docker compose -f docker-compose.prod.yml exec postgres \
  psql -U safrat -d safrat -c "SELECT COUNT(*) FROM operators;"
```

---

## 13. Security Checklist Before Go-Live

Full hashing/encryption audit run 2026-08-16, covering password storage,
session tokens, transport, secrets handling, and API/callback response
leakage. **Already production-grade, verified live, no action needed:**

- **Password hashing** — Better Auth hashes with `scrypt` (Node's native
  `crypto.scrypt`, salted per-password), not a weaker/faster hash. Minimum
  length 8 is enforced server-side (`emailAndPassword.minPasswordLength`),
  not just the frontend's `<input minLength>` — a direct API call can't
  bypass it.
- **Google OAuth** — uses PKCE (`code_challenge`/`S256`) and Better Auth's
  built-in `state` param, confirmed live against a real Google consent
  screen. `account_not_linked` correctly blocks an unverified local
  account from being silently hijacked by a same-email Google sign-in.
- **Brute-force protection** — Better Auth's built-in rate limiter on its
  own endpoints (sign-in/sign-up) auto-enables when `NODE_ENV=production`
  (already set in the Dockerfile, §6). The app's own Connect-RPC rate
  limiter (`internal/middleware/ratelimit.go`) separately covers the
  public pilgrim/agent endpoints.
- **Session security** — single-session enforced (signing in anywhere
  revokes every other session for that user, no grace period); session
  tokens are Better Auth's own cryptographically random strings, never
  logged (`logging()` in `main.go` only logs method/path/duration, never
  headers or bodies).
- **Secrets** — `.env.example` is a committed template; every real secret
  (Google OAuth, `BETTER_AUTH_SECRET`, Sentry DSNs) lives only in
  gitignored `.env.local`/`.env`. Nothing hardcoded in source (audited via
  full-repo grep).
- **CORS** — the configured canonical origin plus structurally validated,
  first-level tenant origins on the same scheme/base domain. Arbitrary,
  nested, wrong-scheme, and lookalike origins are rejected; no `*` response is
  used.
- **API responses** — unmapped internal errors are reported to Sentry but
  never returned to the client; the caller only ever sees a generic
  "internal error" (`serviceError` in `internal/service/errors.go`), never
  a raw Go/SQL error string.
- **Transport** — nginx (§7) terminates TLS, redirects HTTP→HTTPS, and now
  sends `Strict-Transport-Security` + `Referrer-Policy` on both server
  blocks. Postgres/Redis are Docker-network-internal only, never
  host-exposed (see port map at the top of this doc).

**Known, accepted trade-off:**

- `/pilgrim/[code]` and `/leader/[groupId]` carry an unguessable UUID
  directly in the URL path — deliberate, for a one-tap link a jamaah opens
  from WhatsApp without a password. §7's nginx config now disables access
  logging on those paths so the identifier doesn't end up sitting in a log
  file on disk; the UUID itself has 122 bits of entropy
  (`gen_random_uuid()`), so this is about defense-in-depth, not a broken
  primitive.

**Closed since the initial audit (2026-08-16):**

- **Password reset + email verification** — both link-based, via Resend
  (`lib/email.ts`, wired into `lib/auth.ts`). `requireEmailVerification: true`
  means an unverified account can no longer sign in and use the app
  indefinitely — this is also what gives account-linking's
  `requireLocalEmailVerified` guard (below) real teeth for an organically
  signed-up account, not just a Google-first one. `/forgot-password` and
  `/reset-password` are the user-facing pages. Verified live against a
  real Resend send (see commit) — RESEND_API_KEY must still be set per
  environment (`.env.local` for dev, `.env.prod` for production) or these
  are silent no-ops, same as Firebase/Sentry.

**Not yet built — do before handling real payment data (Module 7):**

- **PII stored unencrypted at rest** — passport numbers, phone numbers,
  emergency contacts are plain columns in `pilgrims`. Column-level
  encryption is a real architecture decision (key management, and it
  breaks plain `WHERE passport_number = ...` search/sort unless paired
  with a blind index) — flagging for an explicit decision, not
  implementing unprompted.

---

*Last updated: August 2026 — ports 9100 (API) / 9101 (Web) · Better Auth replaces Clerk*
