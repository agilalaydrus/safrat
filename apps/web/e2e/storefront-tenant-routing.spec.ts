import { expect, test } from "@playwright/test";
import { tenantURL } from "./audio-setup";
import { loadWebEnv } from "./fixture";

loadWebEnv();

/**
 * Better Auth's client is pinned to NEXT_PUBLIC_APP_URL (the apex). A relative
 * /sign-in href on a tenant storefront keeps the visitor on the tenant host,
 * where every /api/auth call is cross-origin and blocked by CORS — a sign-in
 * form that cannot sign anyone in. Every auth link must therefore be absolute
 * to the apex.
 *
 * Note this does NOT cover someone typing vacana.tawafiqhub.id/sign-in
 * directly; that page still renders and is still broken. Fixing that needs a
 * cross-host redirect from middleware, which cannot be verified locally
 * because Next normalizes the request host to localhost in dev and collapses
 * cross-origin Location headers to a bare path.
 */
const apexOrigin = process.env.E2E_APP_URL ?? "http://127.0.0.1:3131";

test("every auth link on the tenant storefront points at the apex", async ({ page }) => {
  await page.goto(tenantURL, { waitUntil: "domcontentloaded" });

  const signInLinks = page.locator('a[href*="/sign-in"]');
  const count = await signInLinks.count();
  expect(count, "the storefront should expose at least one sign-in link").toBeGreaterThan(0);

  for (let index = 0; index < count; index += 1) {
    const href = await signInLinks.nth(index).getAttribute("href");
    expect(href, "auth links must be absolute, not relative to the tenant host").toMatch(/^https?:\/\//);
    expect(new URL(href!).origin).toBe(new URL(apexOrigin).origin);
  }
});

test("the portal access dialog's links also point at the apex", async ({ page }) => {
  await page.goto(tenantURL, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "Daftar" }).first().click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  const links = dialog.locator('a[href*="/sign-in"]');
  const count = await links.count();
  expect(count).toBeGreaterThan(0);
  for (let index = 0; index < count; index += 1) {
    const href = await links.nth(index).getAttribute("href");
    expect(new URL(href!).origin).toBe(new URL(apexOrigin).origin);
  }
});
