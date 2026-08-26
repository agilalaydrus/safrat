# Browser end-to-end specs

These drive the **real** local stack — Next.js, the Go API, PostgreSQL and
MinIO — through a real browser. They exist because the storefront CMS, media
upload, background audio and offline PWA work had never been exercised through
a browser at all; only through Go tests that bypass both the API and the UI.

They are **not** part of `pnpm lint` / `pnpm typecheck` / CI: they need running
services and they write to the local database.

## What they cover

| Spec | What it proves |
| --- | --- |
| `storefront-cms.spec.ts` | A real image upload runs the whole chain — presign → browser WebP conversion → PUT → HEAD/signature/dimension verification → promotion → registry row — then the object is publicly readable and counts against the quota. Draft saves stay out of the published snapshot until publish. |
| `audio-allowed.spec.ts` | A real MP3 upload passes the API's signature check, autoplays where policy permits, and the visitor's mute choice survives a reload. |
| `audio-blocked.spec.ts` | A rejected `play()` falls back to the play control and recovers on interaction. |
| `offline-pwa.spec.ts` | The service worker installs, precaches every PWA route, and serves a fresh tab entirely from cache with the network down. |

## Running them

Everything except the offline project runs against the normal dev servers:

```bash
docker compose up -d postgres redis minio   # if not already up
cd apps/api && go run ./cmd/server           # :8131, needs the S3_* vars in apps/api/.env
pnpm --filter @hajj-saas/web dev --port 3131

pnpm --filter @hajj-saas/web e2e --project=cms
pnpm --filter @hajj-saas/web e2e --project=audio-autoplay-allowed
pnpm --filter @hajj-saas/web e2e --project=audio-autoplay-blocked
```

The offline project needs a **production** build, because Serwist is disabled
under Turbopack. It gets its own `distDir` and its own API origin, since the dev
server would otherwise clobber `.next` and the API allows exactly one CORS
origin:

```bash
pnpm --filter @hajj-saas/web e2e:pwa:api     # Go API on :8141, CORS for :3141
pnpm --filter @hajj-saas/web e2e:pwa:build   # builds into .next-e2e
pnpm --filter @hajj-saas/web e2e:pwa:serve   # serves it on :3141
pnpm --filter @hajj-saas/web e2e --project=offline-pwa
```

`pnpm --filter @hajj-saas/web e2e` runs all of them, which assumes every service
above is up.

> **A plain `pnpm build` breaks the offline project.** `swDest` is
> `public/sw.js` (see `next.config.ts`), which is shared by every build — so a
> normal build overwrites the service worker with a new revision pointing at
> `.next` assets that the `:3141` server does not serve. The worker then fails
> to install and the offline specs fail with `ERR_INTERNET_DISCONNECTED`, which
> looks exactly like a real regression. Re-run `e2e:pwa:build` after any
> `pnpm build`.

## Fixtures

The `setup` project provisions two accounts through the app's own endpoints —
Better Auth over HTTP and the same Connect RPC the onboarding wizard calls —
rather than inserting rows by hand:

- **`e2e-operator@safrat.local`** owns the `e2e-fixture` operator. Everything the
  CMS and audio specs create hangs off it, so runs never touch real local data.
- **`e2e-pilgrim@safrat.local`** is a pilgrim linked to that operator. The
  `/pilgrim` PWA is where the service worker registers, and `RequireAccess`
  bounces anyone who is not a linked pilgrim.

Both are idempotent, and `pnpm --filter @hajj-saas/web e2e:clean` removes them.

The one shortcut is marking the address verified straight in the database:
`requireEmailVerification` is on and no mail is delivered locally.

## Known limits

- **Autoplay blocking is injected, not real.** Headless Chromium does not enforce
  the autoplay policy and no launch flag reinstates it, so `audio-blocked` patches
  `play()` to reject once. It faithfully tests our fallback logic, not the
  browser's policy.
- **"Cold start" keeps one tab open.** Closing every client terminates the worker,
  and Playwright's offline emulation then fails the next navigation instead of
  restarting it. Whether that is an emulation artifact or real Chromium behaviour
  is unresolved.
- **Real devices are still unverified.** iOS Safari audio autoplay and a genuinely
  installed PWA reopened offline both still need a human with a phone.
