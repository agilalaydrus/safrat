import { defineConfig, devices } from "@playwright/test";

// These specs drive the REAL local stack — the Next.js app on :3131, the Go API
// on :8131, PostgreSQL, and MinIO. They are deliberately not part of `pnpm
// lint`/`typecheck`/CI: they need those services running and they write to the
// local database. Run them with `pnpm e2e` (see package.json).
//
// Everything is scoped to a dedicated fixture operator (see e2e/fixture.ts), so
// a run never touches real local data.
const appURL = process.env.E2E_APP_URL ?? "http://127.0.0.1:3131";

export default defineConfig({
  testDir: "./e2e",
  // The fixture account is single-session (see the databaseHooks.session.create
  // hook in lib/auth.ts — signing in anywhere kills every other session for
  // that user), so specs sharing it must not run concurrently.
  workers: 1,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? "list" : [["list"], ["html", { open: "never" }]],
  timeout: 60_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: appURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
    },
    {
      name: "cms",
      dependencies: ["setup"],
      testMatch: /storefront-.*\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], storageState: "./e2e/.auth/operator.json" },
    },
    {
      // The jamaah-facing history, which needs the pilgrim identity rather
      // than operator staff — RequireAccess bounces a staff account from the
      // pilgrim app.
      name: "portal-screens",
      dependencies: ["setup"],
      testMatch: /portal-screens\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], storageState: "./e2e/.auth/pilgrim.json" },
    },
    {
      // The money screens had never been opened in a browser — see the spec's
      // header. Operator storage state, since every one of them is reached
      // from a signed-in staff account.
      name: "money-screens",
      dependencies: ["setup"],
      testMatch: /money-screens\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], storageState: "./e2e/.auth/operator.json" },
    },
    {
      // Autoplay is a browser policy. The permitted branch is real (Chromium
      // launches with the policy disabled); the blocked branch injects the
      // rejection, because headless Chromium will not enforce it.
      name: "audio-autoplay-allowed",
      testMatch: /audio-allowed\.spec\.ts/,
      dependencies: ["setup"],
      use: {
        ...devices["Desktop Chrome"],
        storageState: "./e2e/.auth/operator.json",
        launchOptions: { args: ["--autoplay-policy=no-user-gesture-required"] },
      },
    },
    {
      name: "audio-autoplay-blocked",
      testMatch: /audio-blocked\.spec\.ts/,
      dependencies: ["setup"],
      use: {
        ...devices["Desktop Chrome"],
        storageState: "./e2e/.auth/operator.json",
        // No launch flag reinstates the autoplay policy in headless Chromium,
        // so this spec injects the rejection itself — see its header comment.
      },
    },
    {
      // Serwist only compiles app/sw.ts into public/sw.js during a production
      // build, so this project runs against `pnpm e2e:pwa:build` +
      // `pnpm e2e:pwa:serve` (:3141) plus `pnpm e2e:pwa:api` (:8141), not the
      // dev server. That build gets its own distDir and its own API origin
      // because the API allows exactly one CORS origin.
      //
      // The worker is registered from the /pilgrim layout, which RequireAccess
      // bounces unless the account is a linked pilgrim — hence the pilgrim
      // storage state rather than the operator's.
      name: "offline-pwa",
      testMatch: /offline-.*\.spec\.ts/,
      dependencies: ["setup"],
      // Installing the worker and filling a 135-entry precache in a cold
      // browser profile routinely outlasts the default per-test timeout.
      timeout: 180_000,
      use: {
        ...devices["Desktop Chrome"],
        storageState: "./e2e/.auth/pilgrim.json",
        baseURL: process.env.E2E_PWA_URL ?? "http://127.0.0.1:3141",
      },
    },
  ],
});
