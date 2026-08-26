import { expect, test } from "@playwright/test";
import { tenantURL } from "./audio-setup";
import { loadWebEnv } from "./fixture";

loadWebEnv();

/**
 * The header must stay pinned while the page scrolls. It silently did not:
 * .tenant-scope carried overflow-x: hidden, which forces overflow-y to compute
 * as auto and turns the element into a scroll container. position: sticky then
 * anchors to that box rather than the viewport, so the header scrolled away
 * with the content. Nothing about the CSS looked wrong — only its geometry gave
 * it away, which is why this asserts the measured position.
 */
test("the header stays pinned while the page scrolls", async ({ page }) => {
  await page.goto(tenantURL, { waitUntil: "domcontentloaded" });
  await page.evaluate(() => { document.documentElement.style.scrollBehavior = "auto"; });

  const header = page.locator("header.tenant-nav");
  await expect(header).toBeVisible();

  for (const offset of [1200, 2500, 4000]) {
    await page.evaluate((y) => window.scrollTo(0, y), offset);
    await page.waitForTimeout(300);
    const top = await header.evaluate((el) => el.getBoundingClientRect().top);
    expect(top, `header should stay at viewport top after scrolling ${offset}px`).toBeCloseTo(0, 0);
  }

  // The scope must never become a scroll container, or sticky breaks again.
  const overflow = await page.locator("main.tenant-scope").evaluate((el) => {
    const style = getComputedStyle(el);
    return { x: style.overflowX, y: style.overflowY };
  });
  expect(overflow.y, "overflow-y must stay visible; auto/scroll re-breaks sticky").toBe("visible");
  expect(overflow.x).not.toBe("hidden");
});

test("the nav switches to its solid state once past the hero", async ({ page }) => {
  await page.goto(tenantURL, { waitUntil: "domcontentloaded" });
  const header = page.locator("header.tenant-nav");
  await expect(header).toHaveClass(/is-transparent/);

  await page.evaluate(() => { document.documentElement.style.scrollBehavior = "auto"; window.scrollTo(0, 2500); });
  await expect(header).toHaveClass(/is-scrolled/);
});
