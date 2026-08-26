import { expect, test } from "@playwright/test";
import { ensurePublishedMusic, musicControl, tenantURL } from "./audio-setup";
import { loadWebEnv } from "./fixture";

loadWebEnv();

/**
 * Headless Chromium does not enforce the autoplay policy, and no launch flag
 * reinstates it, so the rejection is injected instead: the first play() call
 * rejects with NotAllowedError exactly as a real policy would, and later calls
 * (the ones a user gesture triggers) succeed.
 *
 * That makes this a faithful test of OUR fallback logic, not of the browser's
 * policy. Real-device autoplay behaviour — iOS Safari especially — still needs
 * a human with a phone.
 */
test("a blocked autoplay falls back to the play control and recovers on interaction", async ({ page }) => {
  await ensurePublishedMusic(page);

  await page.addInitScript(() => {
    const play = HTMLMediaElement.prototype.play;
    let blocked = false;
    HTMLMediaElement.prototype.play = function patchedPlay(this: HTMLMediaElement) {
      if (!blocked) {
        blocked = true;
        const error = new DOMException("play() failed because the user didn't interact with the document first", "NotAllowedError");
        return Promise.reject(error);
      }
      return play.call(this);
    };
  });

  await page.goto(tenantURL);
  const audio = page.locator("audio");
  await expect(audio).toHaveCount(1);

  // The rejection is caught: the control switches to the blocked/play state
  // instead of the page silently pretending music is running.
  await expect(musicControl(page)).toHaveAttribute("aria-label", "Putar musik latar", { timeout: 20_000 });
  await expect(musicControl(page)).toHaveClass(/is-blocked/);
  expect(await audio.evaluate((element: HTMLAudioElement) => element.paused)).toBe(true);

  // A real interaction is what unblocks it.
  await musicControl(page).click();
  await expect
    .poll(async () => audio.evaluate((element: HTMLAudioElement) => !element.paused), { timeout: 20_000 })
    .toBe(true);
  await expect(musicControl(page)).not.toHaveClass(/is-blocked/);
});
