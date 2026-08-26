import { expect, test } from "@playwright/test";

/**
 * Serwist only compiles app/sw.ts into public/sw.js during a production build,
 * so this project runs against `next build && next start` (E2E_PWA_URL,
 * default :3141) rather than the Turbopack dev server.
 *
 * "Cold start" here means a page that has never been open before, navigating
 * while the network is down — the case the precache exists for.
 *
 * useRegisterShellServiceWorker only runs from the /pilgrim and /leader
 * layouts, so the first visit has to be one of those rather than "/", exactly
 * as a real visitor installing the PWA would.
 */
test.describe.configure({ mode: "serial" });

const precachedRoutes = ["/pilgrim", "/pilgrim/schedule", "/leader", "/leader/check-in"];

test("the service worker installs and precaches every PWA route", async ({ page }) => {
  await page.goto("/pilgrim");

  await page.waitForFunction(async () => {
    const registration = await navigator.serviceWorker.getRegistration();
    return registration?.active?.state === "activated";
  }, undefined, { timeout: 60_000 });

  // The precache is populated asynchronously during install; wait for the
  // routes themselves rather than assuming install implies a filled cache.
  await expect
    .poll(
      async () =>
        page.evaluate(async (routes) => {
          const results = await Promise.all(routes.map((route) => caches.match(route, { ignoreSearch: true })));
          return results.filter(Boolean).length;
        }, precachedRoutes),
      { timeout: 60_000 },
    )
    .toBe(precachedRoutes.length);
});

test("a fresh tab opens precached routes with the network down", async ({ page, context }) => {
  // Install the worker first, exactly as a real first visit does.
  await page.goto("/pilgrim");
  await page.waitForFunction(async () => {
    const registration = await navigator.serviceWorker.getRegistration();
    return registration?.active?.state === "activated";
  }, undefined, { timeout: 60_000 });
  await expect
    .poll(async () => page.evaluate(async () => Boolean(await caches.match("/pilgrim", { ignoreSearch: true }))), { timeout: 60_000 })
    .toBe(true);
  await expect.poll(async () => page.evaluate(() => Boolean(navigator.serviceWorker.controller))).toBe(true);

  await context.setOffline(true);

  // A brand new tab: nothing in its memory or history, every byte from cache.
  //
  // NOTE: this deliberately keeps the first tab open. Closing every client
  // terminates the worker, and Playwright's offline emulation then fails the
  // next navigation with ERR_INTERNET_DISCONNECTED instead of restarting it.
  // Whether that is an emulation artifact or real Chromium behaviour cannot be
  // settled here — a true close-everything-then-reopen-offline check still
  // needs a real installed PWA on a real device.
  const fresh = await context.newPage();
  for (const route of ["/pilgrim", "/pilgrim/schedule", "/leader"]) {
    const response = await fresh.goto(route, { waitUntil: "domcontentloaded" });
    // Served by the worker from the precache, not by the network and not by
    // the browser's offline error page.
    expect(response?.status(), `${route} should be served from the precache`).toBe(200);
    expect(response?.fromServiceWorker(), `${route} should come from the service worker`).toBe(true);
    await expect(fresh.locator("body")).not.toBeEmpty();
  }

  // Reloading a controlled tab offline works too — the returning-visitor case.
  const reloaded = await page.reload({ waitUntil: "domcontentloaded" });
  expect(reloaded?.fromServiceWorker()).toBe(true);

  await context.setOffline(false);
});
