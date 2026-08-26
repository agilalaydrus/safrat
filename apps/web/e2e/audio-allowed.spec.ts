import { expect, test } from "@playwright/test";
import { ensurePublishedMusic, musicControl, tenantURL } from "./audio-setup";
import { loadWebEnv } from "./fixture";

loadWebEnv();

// This project launches Chromium with --autoplay-policy=no-user-gesture-required.
test("background music autoplays when the browser policy permits it", async ({ page }) => {
  await ensurePublishedMusic(page);

  await page.goto(tenantURL);
  const audio = page.locator("audio");
  await expect(audio).toHaveCount(1);

  await expect
    .poll(async () => audio.evaluate((element: HTMLAudioElement) => !element.paused), { timeout: 20_000 })
    .toBe(true);

  // Playing and unmuted, so the control offers to mute.
  await expect(musicControl(page)).toHaveAttribute("aria-label", "Bisukan musik");
  await expect(musicControl(page)).not.toHaveClass(/is-blocked/);

  // Muting is remembered on the device.
  await musicControl(page).click();
  await expect(musicControl(page)).toHaveAttribute("aria-label", "Nyalakan suara musik");
  expect(await page.evaluate(() => window.localStorage.getItem("tawafiq-storefront-music-muted"))).toBe("true");

  await page.reload();
  await expect
    .poll(async () => page.locator("audio").evaluate((element: HTMLAudioElement) => element.muted), { timeout: 20_000 })
    .toBe(true);
});
