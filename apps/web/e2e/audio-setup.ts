import { expect, type Page } from "@playwright/test";
import path from "node:path";
import { fixture, operatorID, query } from "./fixture";

const musicFixture = path.join(__dirname, "fixtures", "silence.mp3");

/** The tenant storefront's own hostname. `.localhost` resolves to loopback. */
export const tenantURL = process.env.E2E_TENANT_URL ?? `http://${fixture.operatorSlug}.localhost:3131`;

/**
 * Makes sure the fixture operator has background music uploaded, enabled, and
 * published, so the storefront actually renders an <audio> element. Idempotent:
 * a second run finds the published snapshot already carrying the track.
 */
export async function ensurePublishedMusic(page: Page): Promise<void> {
  const operator = await operatorID();
  const published = await query<{ published: string | null }>(
    "SELECT published::text AS published FROM operator_storefronts WHERE operator_id = $1",
    [operator],
  );
  const snapshot = published[0]?.published ?? "";
  if (snapshot.includes('"backgroundMusicEnabled":true') && /"backgroundMusicUrl":"http[^"]+\.mp3"/.test(snapshot)) {
    return;
  }

  await page.goto("/dashboard/settings");
  await expect(page.getByText("Landing page travel")).toBeVisible();
  await page.getByRole("button", { name: /Sosial & Audio|Sosial &amp; Audio/ }).click();

  const enable = page.locator('label:has-text("Aktifkan musik latar") input[type="checkbox"]');
  await expect(enable).toBeVisible();
  if (!(await enable.isChecked())) await enable.check();

  const musicField = page.locator('label:has(> span:text-is("File musik"))');
  const current = musicField.locator("audio");
  if ((await current.count()) === 0) {
    await musicField.locator('input[type="file"]').setInputFiles(musicFixture);
    await expect(current).toHaveAttribute("src", /\/storefront\/.+\.mp3$/, { timeout: 45_000 });
  }

  await page.getByRole("button", { name: /Publikasikan/ }).click();
  await expect(page.getByText(/berhasil dipublikasikan/)).toBeVisible();
}

/** The floating play/mute control the storefront renders for background music. */
export function musicControl(page: Page) {
  return page.locator("button.tenant-music-control");
}
