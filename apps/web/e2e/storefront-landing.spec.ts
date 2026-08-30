import { expect, test } from "@playwright/test";

/**
 * The apex landing page after its redesign. Nothing covered it before, so a
 * section could disappear in a refactor and only be noticed by a visitor.
 *
 * Sections are addressed by their anchor ids rather than their copy: the
 * wording is marketing and will change, the anchors are what the navigation
 * links to and what a broken page loses.
 */
test.describe("landing page", () => {
  test("every section is present and nothing throws", async ({ page }) => {
    const failures: string[] = [];
    page.on("pageerror", (error) => failures.push(`pageerror: ${error.message}`));
    page.on("console", (message) => {
      if (message.type() === "error") failures.push(`console: ${message.text()}`);
    });
    page.on("response", (response) => {
      if (response.status() >= 400) failures.push(`${response.status()}: ${response.url()}`);
    });

    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    // The four the navigation points at. A missing anchor is a nav link that
    // scrolls nowhere, which reads as a broken site rather than a missing
    // section.
    for (const id of ["platform", "solusi", "cara-kerja", "harga"]) {
      await expect(page.locator(`#${id}`)).toHaveCount(1);
    }

    expect(failures, failures.join("\n")).toHaveLength(0);
  });

  test("the page does not scroll sideways on a phone", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/");
    // Horizontal overflow is the failure that survives review and reaches
    // every visitor on a phone, because nobody checks for it on a laptop.
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow, `halaman meluber ${overflow}px ke samping`).toBeLessThanOrEqual(0);
  });
});
