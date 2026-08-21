# Safrat — VPS Deployment Guide

> **Stack:** Ubuntu 22.04 LTS · Docker Compose · nginx · Let's Encrypt · GitHub Actions
>
> **Port map (host-facing, bound to 127.0.0.1 only):**
> - `9100` → Go API (container internal: 8080)
> - `9101` → Next.js Web (container internal: 3000)
> - `5432` NOT exposed — PostgreSQL stays inside Docker network only
> - `6379` NOT exposed — Redis stays inside Docker network only (§4);
>   only `worker` reads it, the API server never touches Redis directly
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
      DATABASE_URL: postgresql://safrat:${POSTGRES_PASSWORD}@postgres:5432/safrat
      BETTER_AUTH_SECRET: ${BETTER_AUTH_SECRET}
      CORS_ALLOWED_ORIGIN: https://app.safrat.com
      SENTRY_DSN: ${SENTRY_DSN}
      FIREBASE_SERVICE_ACCOUNT_JSON: ${FIREBASE_SERVICE_ACCOUNT_JSON}
      XENDIT_SECRET_KEY: ${XENDIT_SECRET_KEY}
      XENDIT_WEBHOOK_TOKEN: ${XENDIT_WEBHOOK_TOKEN}
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
      DATABASE_URL: postgresql://safrat:${POSTGRES_PASSWORD}@postgres:5432/safrat
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
      DATABASE_URL: postgresql://safrat:${POSTGRES_PASSWORD}@postgres:5432/safrat
      BETTER_AUTH_SECRET: ${BETTER_AUTH_SECRET}
      BETTER_AUTH_URL: https://app.safrat.com
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
# reaches the Go API directly. Redirect URI: https://app.safrat.com/api/auth/callback/google
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...

# Redis — self-hosted via the `redis` compose service above, not Upstash;
# only cmd/worker reads REDIS_URL (the API server never touches Redis)
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
RESEND_FROM_EMAIL=noreply@safrat.com

# Xendit (Module 7 — Orders & Payments) — apps/api only, unrelated to
# email/web. Without XENDIT_SECRET_KEY, checkout fails fast with a clear
# error (internal/payment/xendit.go) rather than creating an unpayable
# order. Register the webhook URL in the Xendit Dashboard > Settings >
# Webhooks: https://api.safrat.com/webhooks/xendit — XENDIT_WEBHOOK_TOKEN
# must match the "Verification Token" shown on that same page.
XENDIT_SECRET_KEY=xnd_production_...
XENDIT_WEBHOOK_TOKEN=...

# Observability — optional, no-op on both api and web when unset
SENTRY_DSN=...
NEXT_PUBLIC_SENTRY_DSN=...

# Cloudflare R2 — reserved for future file storage (product images, PDF
# exports); not read by any code path yet, nothing consumes these today.
# R2_ACCOUNT_ID=...
# R2_ACCESS_KEY_ID=...
# R2_SECRET_ACCESS_KEY=...
# R2_BUCKET_NAME=safrat-uploads
```

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
# POSTGRES_PASSWORD is percent-encoded before going into DATABASE_URL — any
# of @ / # " \ etc. in the raw password breaks URL parsing otherwise (this
# broke a real deploy: pg-connection-string threw "Invalid URL").
docker run --rm \
  --network safrat_internal \
  -v $(pwd):/repo \
  -w /repo \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  node:20-alpine \
  sh -c 'apk add --no-cache python3 make g++ && ENCODED_PW=$(node -e "console.log(encodeURIComponent(process.env.POSTGRES_PASSWORD))") && export DATABASE_URL="postgresql://safrat:${ENCODED_PW}@postgres:5432/safrat" && corepack enable && corepack prepare pnpm@9 --activate && pnpm install --frozen-lockfile --config.node-linker=hoisted && cd apps/web && npx @better-auth/cli@1.4.21 migrate --yes'

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

**Step 1 — HTTP only first** (before SSL):

```nginx
server {
    listen 80;
    server_name api.safrat.com;
    location / {
        proxy_pass http://127.0.0.1:9100;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
server {
    listen 80;
    server_name app.safrat.com;
    location / {
        proxy_pass http://127.0.0.1:9101;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/safrat /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

**Step 2 — Issue SSL:**

```bash
sudo certbot --nginx -d app.safrat.com -d api.safrat.com
```

**Step 3 — Full TLS config** (replace after cert issued):

```nginx
# API — Go backend
server {
    listen 80;
    server_name api.safrat.com;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl http2;
    server_name api.safrat.com;

    ssl_certificate     /etc/letsencrypt/live/api.safrat.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.safrat.com/privkey.pem;

    add_header X-Frame-Options SAMEORIGIN;
    add_header X-Content-Type-Options nosniff;
    add_header Referrer-Policy strict-origin-when-cross-origin;
    # Pins the browser to HTTPS for a year (incl. subdomains) after the first
    # successful HTTPS response — closes the window where a plain-HTTP
    # request (e.g. a stale bookmark, a typed URL without https://) would
    # otherwise carry the Bearer session token in the clear before the 301.
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    location / {
        proxy_pass         http://127.0.0.1:9100;
        proxy_http_version 1.1;
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        # Connect streaming RPCs
        proxy_buffering    off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}

# Web — Next.js
server {
    listen 80;
    server_name app.safrat.com;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl http2;
    server_name app.safrat.com;

    ssl_certificate     /etc/letsencrypt/live/app.safrat.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/app.safrat.com/privkey.pem;

    add_header X-Frame-Options SAMEORIGIN;
    add_header X-Content-Type-Options nosniff;
    add_header Referrer-Policy strict-origin-when-cross-origin;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # /pilgrim/[code] and /leader/[groupId] put a bearer-equivalent
    # identifier (app_access_code / session-adjacent group id) directly in
    # the URL path — a deliberate trade-off for a one-tap link a jamaah can
    # open from WhatsApp without typing a password. nginx's default combined
    # log format writes the full request path to disk, so those values land
    # in plaintext in this server's access logs. Turn access logging off for
    # those two paths (or route them to a log with restricted permissions)
    # before this box holds real jamaah data — the identifiers themselves
    # are unguessable UUIDs, but a leaked log file defeats that.
    location ~ ^/(pilgrim|leader)/ {
        access_log off;
        proxy_pass         http://127.0.0.1:9101;
        proxy_http_version 1.1;
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }

    # PWA service worker — never cache
    location = /sw.js {
        proxy_pass http://127.0.0.1:9101;
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        proxy_set_header Host $host;
    }

    location / {
        proxy_pass         http://127.0.0.1:9101;
        proxy_http_version 1.1;
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
```

```bash
sudo nginx -t && sudo systemctl reload nginx
sudo certbot renew --dry-run
sudo systemctl status certbot.timer
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

# SSL cert expiry
sudo certbot certificates

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
- **CORS** — exact-origin allowlist (`cors()` in `main.go`), not a
  wildcard or reflected-origin.
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
